package migrationimport

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// XLSX decompression bombs are bounded before parsing: a valid import file
// (≤10 MiB compressed) never expands beyond these limits, while a crafted
// zip bomb is rejected instead of exhausting memory.
const (
	// xlsxUnzipSizeLimit caps the total decompressed size of the archive.
	xlsxUnzipSizeLimit = 128 << 20
	// xlsxUnzipXMLSizeLimit caps a single worksheet / sharedStrings XML part.
	xlsxUnzipXMLSizeLimit = 64 << 20
)

// parseXLSX reads the first worksheet of an .xlsx file into a string table
// using excelize (BSD-3-Clause). Formulas resolve to their cached values;
// dates and numbers are returned as displayed text.
func parseXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    xlsxUnzipSizeLimit,
		UnzipXMLSizeLimit: xlsxUnzipXMLSizeLimit,
	})
	if err != nil {
		if err == excelize.ErrWorkbookFileFormat || err == excelize.ErrWorkbookPassword {
			return nil, fmt.Errorf("XLSX 解析失败：不是有效的 xlsx 文件（加密文件请先另存为未加密版本）")
		}
		return nil, fmt.Errorf("XLSX 解析失败：文件损坏或超出解压安全限制")
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("XLSX 中未找到工作表")
	}
	rowIter, err := f.Rows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("XLSX 工作表读取失败")
	}
	defer rowIter.Close()
	var table [][]string
	contentRows := 0
	for rowIter.Next() {
		cells, err := rowIter.Columns()
		if err != nil {
			return nil, fmt.Errorf("XLSX 工作表解析失败")
		}
		table = append(table, cells)
		if rowHasContent(cells) {
			contentRows++
		}
		// Header + data-row cap: stop early on oversized files instead of
		// loading them fully into memory (blank rows do not count).
		if contentRows > MaxImportRows+1 {
			return nil, fmt.Errorf("单批最多导入 %d 行数据，请拆分文件", MaxImportRows)
		}
	}
	if err := rowIter.Error(); err != nil {
		return nil, fmt.Errorf("XLSX 工作表解析失败")
	}
	return table, nil
}
