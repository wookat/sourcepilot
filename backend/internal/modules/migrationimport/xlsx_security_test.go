package migrationimport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestParseXLSXCorruptedFile(t *testing.T) {
	cases := map[string][]byte{
		"not a zip":     []byte("this is not a zip file at all"),
		"empty zip":     buildZip(t, map[string]string{}),
		"zip no sheet":  buildZip(t, map[string]string{"hello.txt": "hi"}),
		"truncated":     buildTestXLSX(t)[:64],
		"malformed xml": buildZip(t, map[string]string{"xl/workbook.xml": "<workbook><"}),
	}
	for name, data := range cases {
		if _, err := ParseImportFile("x.xlsx", data); err == nil {
			t.Fatalf("%s: expected parse error", name)
		} else if !strings.Contains(err.Error(), "XLSX") {
			t.Fatalf("%s: expected Chinese XLSX error, got %v", name, err)
		}
	}
}

// TestParseXLSXZipBomb builds a small archive holding a worksheet XML that
// decompresses far beyond the unzip limits and expects rejection instead of
// memory exhaustion.
func TestParseXLSXZipBomb(t *testing.T) {
	// ~200MB of zeros compresses to well under 10MB.
	payload := bytes.Repeat([]byte("0"), 200<<20)
	bomb := buildZip(t, map[string]string{
		"[Content_Types].xml":      `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/></Types>`,
		"xl/workbook.xml":          `<?xml version="1.0"?><workbook><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/worksheets/sheet1.xml": string(payload),
	})
	if len(bomb) > MaxImportFileSize {
		t.Fatalf("compressed bomb should fit the 10MB upload cap, got %d", len(bomb))
	}
	if _, err := ParseImportFile("bomb.xlsx", bomb); err == nil {
		t.Fatal("expected zip bomb rejection")
	}
}

func TestParseXLSXRowLimitStopsEarly(t *testing.T) {
	rows := make([][]interface{}, 0, MaxImportRows+10)
	rows = append(rows, []interface{}{"商品名称"})
	for i := 0; i < MaxImportRows+5; i++ {
		rows = append(rows, []interface{}{"x"})
	}
	if _, err := ParseImportFile("big.xlsx", buildXLSX(t, rows)); err == nil {
		t.Fatal("expected row limit error")
	} else if !strings.Contains(err.Error(), "拆分文件") {
		t.Fatalf("expected Chinese row-limit error, got %v", err)
	}
}

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// FuzzParseImportFile feeds arbitrary bytes through both the CSV and XLSX
// paths; any input may fail parsing but must never panic or hang.
func FuzzParseImportFile(f *testing.F) {
	f.Add([]byte("商品名称,SKU\n测试,SKU-1\n"))
	f.Add([]byte("PK\x03\x04garbage"))
	f.Add([]byte{0x50, 0x4b, 0x05, 0x06, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte(""))
	seedXLSX := func() []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("xl/worksheets/sheet1.xml")
		_, _ = w.Write([]byte(`<worksheet><sheetData><row><c t="inlineStr"><is><t>x</t></is></c></row></sheetData></worksheet>`))
		_ = zw.Close()
		return buf.Bytes()
	}
	f.Add(seedXLSX())
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseImportFile("f.xlsx", data)
		_, _ = ParseImportFile("f.csv", data)
	})
}
