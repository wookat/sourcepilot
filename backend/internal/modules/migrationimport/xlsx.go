package migrationimport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Minimal XLSX (SpreadsheetML) reader for import files: reads the first
// worksheet with shared-string / inline-string / numeric cells. Formulas
// resolve to their cached value; styles and dates are returned as raw text.

type xlsxSST struct {
	SI []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type xlsxSheet struct {
	Rows []struct {
		R     int `xml:"r,attr"`
		Cells []struct {
			R  string `xml:"r,attr"`
			T  string `xml:"t,attr"`
			V  string `xml:"v"`
			IS struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func parseXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("XLSX 解析失败：不是有效的 xlsx 文件")
	}
	var shared []string
	if b, err := readZipFile(zr, "xl/sharedStrings.xml"); err == nil {
		var sst xlsxSST
		if err := xml.Unmarshal(b, &sst); err == nil {
			shared = make([]string, 0, len(sst.SI))
			for _, si := range sst.SI {
				if len(si.R) > 0 {
					var sb strings.Builder
					for _, r := range si.R {
						sb.WriteString(r.T)
					}
					shared = append(shared, sb.String())
					continue
				}
				shared = append(shared, si.T)
			}
		}
	}
	sheetPath := firstSheetPath(zr)
	if sheetPath == "" {
		return nil, fmt.Errorf("XLSX 中未找到工作表")
	}
	b, err := readZipFile(zr, sheetPath)
	if err != nil {
		return nil, fmt.Errorf("XLSX 工作表读取失败")
	}
	var sheet xlsxSheet
	if err := xml.Unmarshal(b, &sheet); err != nil {
		return nil, fmt.Errorf("XLSX 工作表解析失败")
	}
	var table [][]string
	for _, row := range sheet.Rows {
		var cells []string
		for _, c := range row.Cells {
			col := cellColumnIndex(c.R)
			if col < 0 {
				col = len(cells)
			}
			for len(cells) <= col {
				cells = append(cells, "")
			}
			val := c.V
			switch c.T {
			case "s":
				if idx, err := strconv.Atoi(strings.TrimSpace(c.V)); err == nil && idx >= 0 && idx < len(shared) {
					val = shared[idx]
				}
			case "inlineStr":
				val = c.IS.T
			}
			cells[col] = val
		}
		table = append(table, cells)
	}
	return table, nil
}

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, MaxImportFileSize*4))
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}

func firstSheetPath(zr *zip.Reader) string {
	var candidates []string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
			candidates = append(candidates, f.Name)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	for _, c := range candidates {
		if c == "xl/worksheets/sheet1.xml" {
			return c
		}
	}
	return candidates[0]
}

// cellColumnIndex converts a cell reference like "C7" to a zero-based column index.
func cellColumnIndex(ref string) int {
	col := 0
	seen := false
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			col = col*26 + int(r-'A'+1)
			seen = true
			continue
		}
		if r >= 'a' && r <= 'z' {
			col = col*26 + int(r-'a'+1)
			seen = true
			continue
		}
		break
	}
	if !seen {
		return -1
	}
	return col - 1
}
