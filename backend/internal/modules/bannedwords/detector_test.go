package bannedwords_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
)

func TestScanFindsRunePositions(t *testing.T) {
	fields := []bannedwords.FieldText{
		{Field: "title", Label: "商品标题", Text: "全网最低价，销量第一的最佳选择"},
	}
	words := []bannedwords.BannedWord{
		{Word: "全网最低", Category: bannedwords.CategoryAdExtreme, Level: bannedwords.LevelForbidden},
		{Word: "第一", Category: bannedwords.CategoryAdExtreme, Level: bannedwords.LevelForbidden},
		{Word: "最佳", Category: bannedwords.CategoryAdExtreme, Level: bannedwords.LevelForbidden},
		{Word: "治疗", Category: bannedwords.CategoryMedical, Level: bannedwords.LevelForbidden},
	}
	hits := bannedwords.Scan(fields, words)
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d: %+v", len(hits), hits)
	}
	byWord := map[string]bannedwords.Hit{}
	for _, h := range hits {
		byWord[h.Word] = h
	}
	h, ok := byWord["全网最低"]
	if !ok || len(h.Positions) != 1 || h.Positions[0].Start != 0 || h.Positions[0].End != 4 {
		t.Fatalf("unexpected 全网最低 positions: %+v", h.Positions)
	}
	h = byWord["第一"]
	if len(h.Positions) != 1 || h.Positions[0].Start != 8 {
		t.Fatalf("unexpected 第一 positions: %+v", h.Positions)
	}
}

func TestScanIsCaseInsensitiveAndCountsMultiple(t *testing.T) {
	fields := []bannedwords.FieldText{
		{Field: "description", Label: "商品详情/卖点", Text: "a货 A货 正品"},
	}
	words := []bannedwords.BannedWord{
		{Word: "A货", Category: bannedwords.CategoryInfringement, Level: bannedwords.LevelForbidden},
	}
	hits := bannedwords.Scan(fields, words)
	if len(hits) != 1 || len(hits[0].Positions) != 2 {
		t.Fatalf("expected 1 hit with 2 positions, got %+v", hits)
	}
}

func TestScanSkipsEmptyFieldsAndWords(t *testing.T) {
	fields := []bannedwords.FieldText{
		{Field: "title", Label: "商品标题", Text: "   "},
		{Field: "description", Label: "商品详情/卖点", Text: "普通描述"},
	}
	words := []bannedwords.BannedWord{
		{Word: "  ", Category: bannedwords.CategoryGeneral, Level: bannedwords.LevelForbidden},
		{Word: "违禁", Category: bannedwords.CategoryGeneral, Level: bannedwords.LevelForbidden},
	}
	if hits := bannedwords.Scan(fields, words); len(hits) != 0 {
		t.Fatalf("expected no hits, got %+v", hits)
	}
}

func TestScanOrdersForbiddenFirst(t *testing.T) {
	fields := []bannedwords.FieldText{
		{Field: "title", Label: "商品标题", Text: "祖传最佳"},
	}
	words := []bannedwords.BannedWord{
		{Word: "祖传", Category: bannedwords.CategoryAdExtreme, Level: bannedwords.LevelWarning},
		{Word: "最佳", Category: bannedwords.CategoryAdExtreme, Level: bannedwords.LevelForbidden},
	}
	hits := bannedwords.Scan(fields, words)
	if len(hits) != 2 || hits[0].Level != bannedwords.LevelForbidden {
		t.Fatalf("expected forbidden hit first, got %+v", hits)
	}
}
