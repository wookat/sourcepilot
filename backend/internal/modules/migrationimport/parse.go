package migrationimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// MaxImportFileSize caps uploaded import files (10 MiB).
const MaxImportFileSize = 10 << 20

// ParsedFile is the result of parsing one uploaded CSV / XLSX file.
type ParsedFile struct {
	Columns  []string
	Rows     [][]string
	FileHash string
}

// ParseImportFile parses CSV or XLSX bytes into a header row plus data rows.
// CSV accepts UTF-8 (with or without BOM) and falls back to GBK. The first
// non-empty row is treated as the header.
func ParseImportFile(fileName string, data []byte) (*ParsedFile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("文件为空")
	}
	if len(data) > MaxImportFileSize {
		return nil, fmt.Errorf("文件超过 10MB 限制")
	}
	var table [][]string
	var err error
	lower := strings.ToLower(fileName)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		table, err = parseXLSX(data)
	case strings.HasSuffix(lower, ".csv"), strings.HasSuffix(lower, ".txt"), lower == "":
		table, err = parseCSV(data)
	default:
		return nil, fmt.Errorf("仅支持 CSV / XLSX 文件")
	}
	if err != nil {
		return nil, err
	}
	columns, rows := splitHeader(table)
	if len(columns) == 0 {
		return nil, fmt.Errorf("未找到表头行")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("文件没有数据行")
	}
	if len(rows) > MaxImportRows {
		return nil, fmt.Errorf("单批最多导入 %d 行数据（当前 %d 行），请拆分文件", MaxImportRows, len(rows))
	}
	sum := sha256.Sum256(data)
	return &ParsedFile{Columns: columns, Rows: rows, FileHash: hex.EncodeToString(sum[:])}, nil
}

func parseCSV(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), data)
		if err != nil {
			return nil, fmt.Errorf("CSV 编码无法识别（支持 UTF-8 / GBK）")
		}
		data = decoded
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败: %v", err)
	}
	return records, nil
}

func splitHeader(table [][]string) ([]string, [][]string) {
	headerIdx := -1
	for i, row := range table {
		if rowHasContent(row) {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, nil
	}
	columns := make([]string, len(table[headerIdx]))
	for i, c := range table[headerIdx] {
		columns[i] = strings.TrimSpace(c)
	}
	var rows [][]string
	for _, row := range table[headerIdx+1:] {
		if !rowHasContent(row) {
			continue
		}
		norm := make([]string, len(columns))
		for i := range columns {
			if i < len(row) {
				norm[i] = strings.TrimSpace(row[i])
			}
		}
		rows = append(rows, norm)
	}
	return columns, rows
}

func rowHasContent(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
}
