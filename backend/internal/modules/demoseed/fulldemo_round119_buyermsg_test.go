package demoseed

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
)

func TestFullDemoSeedBuyerMessages(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var rules []customerchat.BuyerMessageRule
	if err := db.Find(&rules).Error; err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 demo buyer message rules, got %d", len(rules))
	}
	var disabled int
	for _, r := range rules {
		if r.TenantID != 1 {
			t.Fatalf("rule %s wrong tenant %d", r.Name, r.TenantID)
		}
		if !strings.HasPrefix(r.Name, DemoPrefix) {
			t.Fatalf("rule %s missing DEMO- prefix", r.Name)
		}
		if !r.Enabled {
			disabled++
		}
	}
	if disabled != 1 {
		t.Fatalf("expected exactly one disabled demo rule, got %d", disabled)
	}

	var drafts []customerchat.BuyerMessageDraft
	if err := db.Find(&drafts).Error; err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 6 {
		t.Fatalf("expected 6 demo drafts, got %d", len(drafts))
	}
	statuses := map[string]int{}
	missingSample := 0
	langByOrder := map[string][2]string{}
	for _, d := range drafts {
		if d.TenantID != 1 {
			t.Fatalf("draft %s wrong tenant %d", d.OrderNo, d.TenantID)
		}
		statuses[d.Status]++
		langByOrder[d.OrderNo] = [2]string{d.Language, d.LangSource}
		var miss []string
		if len(d.MissingVars) > 0 {
			miss = jsonList(t, d.MissingVars)
		}
		if len(miss) > 0 {
			missingSample++
		}
	}
	if statuses[customerchat.BuyerMsgDraftPending] != 4 ||
		statuses[customerchat.BuyerMsgDraftSent] != 1 ||
		statuses[customerchat.BuyerMsgDraftIgnored] != 1 {
		t.Fatalf("draft statuses: %+v", statuses)
	}
	if missingSample < 1 {
		t.Fatal("expected at least one draft with missing vars sample")
	}
	if v := langByOrder["DEMO-BM-1005"]; v != [2]string{"en", customerchat.BuyerMsgLangSourceOrderCountry} {
		t.Fatalf("DEMO-BM-1005 language sample: %v", v)
	}
	if v := langByOrder["DEMO-BM-1006"]; v != [2]string{"pt", customerchat.BuyerMsgLangSourceOrderCountry} {
		t.Fatalf("DEMO-BM-1006 language sample: %v", v)
	}
	if v := langByOrder["DEMO-BM-1001"]; v != [2]string{"zh-CN", customerchat.BuyerMsgLangSourceFallback} {
		t.Fatalf("DEMO-BM-1001 fallback sample: %v", v)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["buyer_message_rules"] != 0 {
		t.Fatalf("buyer message rules residue: %d", verify.Counts["buyer_message_rules"])
	}
	if verify.Counts["buyer_message_drafts"] != 0 {
		t.Fatalf("buyer message drafts residue: %d", verify.Counts["buyer_message_drafts"])
	}
	var residue int64
	db.Model(&customerchat.BuyerMessageRule{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("rules residue: %d", residue)
	}
	db.Model(&customerchat.BuyerMessageDraft{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("drafts residue: %d", residue)
	}
}

func jsonList(t *testing.T, raw []byte) []string {
	t.Helper()
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
