package migrationimport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestParseImportFileCSVUTF8BOM(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("商品名称,SKU,售价\n测试商品,SKU-1,9.9\n")...)
	p, err := ParseImportFile("a.csv", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Columns) != 3 || p.Columns[0] != "商品名称" {
		t.Fatalf("columns: %v", p.Columns)
	}
	if len(p.Rows) != 1 || p.Rows[0][1] != "SKU-1" {
		t.Fatalf("rows: %v", p.Rows)
	}
	if p.FileHash == "" {
		t.Fatal("expected file hash")
	}
}

func TestParseImportFileCSVGBK(t *testing.T) {
	gbk, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte("订单号,收件人\nSO-1,张三\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseImportFile("b.csv", gbk)
	if err != nil {
		t.Fatal(err)
	}
	if p.Columns[1] != "收件人" || p.Rows[0][1] != "张三" {
		t.Fatalf("gbk decode failed: %v %v", p.Columns, p.Rows)
	}
}

func TestParseImportFileRowLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("商品名称\n")
	for i := 0; i < MaxImportRows+1; i++ {
		sb.WriteString("x\n")
	}
	if _, err := ParseImportFile("c.csv", []byte(sb.String())); err == nil {
		t.Fatal("expected row limit error")
	}
}

func TestParseImportFileUnsupported(t *testing.T) {
	if _, err := ParseImportFile("a.pdf", []byte("x")); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func buildTestXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("xl/sharedStrings.xml", `<?xml version="1.0"?><sst><si><t>商品名称</t></si><si><t>SKU</t></si><si><t>测试商品</t></si><si><r><t>SKU-</t></r><r><t>9</t></r></si></sst>`)
	write("xl/worksheets/sheet1.xml", `<?xml version="1.0"?><worksheet><sheetData>`+
		`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="inlineStr"><is><t>售价</t></is></c></row>`+
		`<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2" t="s"><v>3</v></c><c r="C2"><v>19.9</v></c></row>`+
		`</sheetData></worksheet>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseImportFileXLSX(t *testing.T) {
	p, err := ParseImportFile("d.xlsx", buildTestXLSX(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Columns) != 3 || p.Columns[2] != "售价" {
		t.Fatalf("columns: %v", p.Columns)
	}
	if p.Rows[0][1] != "SKU-9" || p.Rows[0][2] != "19.9" {
		t.Fatalf("rows: %v", p.Rows)
	}
}

func TestGuessMappingAndDetect(t *testing.T) {
	cols := []string{"订单号", "收货人姓名", "平台订单号", "商品名称", "数量", "订单状态", "详细地址"}
	m := GuessMapping(KindOrder, cols)
	if m[FOrderNo] != 0 || m[FCustomerName] != 1 || m[FQuantity] != 4 || m[FStatus] != 5 {
		t.Fatalf("mapping: %v", m)
	}
	if got := DetectSourceFormat(cols); got != SourceDianxiaomi {
		t.Fatalf("detect dianxiaomi got %s", got)
	}
	mb := []string{"订单号", "交易单号", "库存SKU", "邮寄地址", "数量", "状态"}
	if got := DetectSourceFormat(mb); got != SourceMabang {
		t.Fatalf("detect mabang got %s", got)
	}
	if got := DetectSourceFormat([]string{"a", "b"}); got != SourceCustom {
		t.Fatalf("detect custom got %s", got)
	}
}

func TestMapOrderStatus(t *testing.T) {
	m, ok := MapOrderStatus("已发货")
	if !ok || m.Status != "shipped" {
		t.Fatalf("已发货: %v %v", m, ok)
	}
	if _, ok := MapOrderStatus("神秘状态"); ok {
		t.Fatal("expected unknown status")
	}
}
