package settings

import (
	"context"
	"strings"
	"testing"
)

// R189 线1（收口 R188 线2 P2-1）：mcp/mark_paid_single_limit 与
// mcp/mark_paid_daily_limit 必须在写入时做服务端值域校验。此前
// PUT /api/v1/settings 原样存储任意字符串，存入 1e20 等超大值会使
// mark-paid 单笔/日累计上限实质失效（仅靠 amount ≤ 1e10 兜底）。

func putLimit(t *testing.T, svc *Service, key, val string) error {
	t.Helper()
	return svc.PutBulk(context.Background(), []PutItem{
		{GroupKey: "mcp", ItemKey: key, ItemValue: val},
	})
}

func TestMarkPaidLimitRejectsInvalidValues(t *testing.T) {
	svc := newSettingsTestSvc(t)
	bad := []struct {
		name string
		val  string
	}{
		{"huge scientific 1e20", "1e20"},
		{"huge plain digits", "99999999999999999999"},
		{"just above cap", "10000000000.01"},
		{"negative", "-1"},
		{"zero", "0"},
		{"NaN", "NaN"},
		{"Infinity", "Infinity"},
		{"non numeric string", "abc"},
		{"three decimals", "12.345"},
		{"scientific notation small", "1e5"},
		{"hex", "0x10"},
		{"plus sign", "+500"},
		{"thousands separator", "1,000"},
		{"trailing garbage", "500x"},
	}
	for _, tc := range bad {
		for _, key := range []string{"mark_paid_single_limit", "mark_paid_daily_limit"} {
			if err := putLimit(t, svc, key, tc.val); err == nil {
				t.Errorf("%s: mcp/%s must reject %q at write time", tc.name, key, tc.val)
			}
		}
	}
	// Nothing invalid may have been persisted.
	var n int64
	if err := svc.DB.Model(&Setting{}).Where("group_key = ?", "mcp").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("invalid limit values must not be persisted, found %d rows", n)
	}
}

func TestMarkPaidLimitRejectionIsAtomicWithinBulk(t *testing.T) {
	svc := newSettingsTestSvc(t)
	err := svc.PutBulk(context.Background(), []PutItem{
		{GroupKey: "mcp", ItemKey: "mark_paid_single_limit", ItemValue: "500"},
		{GroupKey: "mcp", ItemKey: "mark_paid_daily_limit", ItemValue: "1e20"},
	})
	if err == nil {
		t.Fatal("bulk write containing an invalid limit must fail")
	}
	var n int64
	if err := svc.DB.Model(&Setting{}).Where("group_key = ?", "mcp").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rejected bulk write must roll back entirely, found %d rows", n)
	}
}

func TestMarkPaidLimitAcceptsValidValues(t *testing.T) {
	svc := newSettingsTestSvc(t)
	good := []string{"0.01", "500", "500.5", "2000.00", " 500 ", "10000000000"}
	for _, val := range good {
		if err := putLimit(t, svc, "mark_paid_single_limit", val); err != nil {
			t.Errorf("valid limit %q rejected: %v", val, err)
		}
	}
	// Trimmed value is what gets stored.
	if err := putLimit(t, svc, "mark_paid_single_limit", " 500 "); err != nil {
		t.Fatal(err)
	}
	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "mcp", "mark_paid_single_limit").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ItemValue != "500" {
		t.Fatalf("limit value must be stored trimmed, got %q", row.ItemValue)
	}
}

func TestMarkPaidLimitAllowsEmptyAndClear(t *testing.T) {
	svc := newSettingsTestSvc(t)
	// Empty value = unset; downstream markPaidLimits fails closed.
	if err := putLimit(t, svc, "mark_paid_daily_limit", ""); err != nil {
		t.Fatalf("empty (unset) limit must be accepted: %v", err)
	}
	if err := svc.PutBulk(context.Background(), []PutItem{
		{GroupKey: "mcp", ItemKey: "mark_paid_daily_limit", ItemValue: "whatever", Clear: true},
	}); err != nil {
		t.Fatalf("clear must be accepted regardless of payload value: %v", err)
	}
}

func TestMarkPaidLimitRejectsEncryptedStorage(t *testing.T) {
	svc := newSettingsTestSvc(t)
	err := svc.PutBulk(context.Background(), []PutItem{
		{GroupKey: "mcp", ItemKey: "mark_paid_single_limit", ItemValue: "500", IsEncrypted: true},
	})
	if err == nil {
		t.Fatal("limit keys must reject encrypted storage (consumer treats encrypted rows as unconfigured)")
	}
	if !strings.Contains(err.Error(), "加密") {
		t.Fatalf("error should explain encryption is not allowed, got: %v", err)
	}
}

func TestMarkPaidLimitErrorIsChineseAndActionable(t *testing.T) {
	svc := newSettingsTestSvc(t)
	err := putLimit(t, svc, "mark_paid_single_limit", "1e20")
	if err == nil {
		t.Fatal("expected rejection")
	}
	msg := err.Error()
	if !strings.Contains(msg, "mark_paid_single_limit") {
		t.Errorf("error must name the offending key, got: %q", msg)
	}
	if !strings.Contains(msg, "两位小数") {
		t.Errorf("error must state the value domain in Chinese, got: %q", msg)
	}
	err = putLimit(t, svc, "mark_paid_single_limit", "99999999999999999999")
	if err == nil {
		t.Fatal("expected rejection above the cap")
	}
	if !strings.Contains(err.Error(), "100 亿") {
		t.Errorf("over-cap error must state the ceiling in Chinese, got: %q", err.Error())
	}
}
