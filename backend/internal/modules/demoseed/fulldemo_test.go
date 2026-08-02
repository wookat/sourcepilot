package demoseed

import (
	"context"
	"strings"
	"testing"
)

func TestDemoPurchaseOrderPlansFollowStateMachine(t *testing.T) {
	plans := demoPurchaseOrderPlans()
	if len(plans) == 0 {
		t.Fatal("no purchase order plans")
	}
	for _, p := range plans {
		if err := validatePurchaseChain(p); err != nil {
			t.Errorf("plan %s: %v", p.suffix, err)
		}
	}
}

func TestDemoSalesOrderPlansFollowLifecycle(t *testing.T) {
	plans := demoSalesOrderPlans()
	if len(plans) != 5 {
		t.Fatalf("expected 5 sales order plans (pending/paid/shipped/delivered/cancelled), got %d", len(plans))
	}
	seen := map[string]bool{}
	for _, p := range plans {
		if err := validateSalesChain(p); err != nil {
			t.Errorf("plan %s: %v", p.suffix, err)
		}
		seen[p.status] = true
	}
	for _, st := range []string{"pending", "paid", "shipped", "delivered", "cancelled"} {
		if !seen[st] {
			t.Errorf("missing sales order status %s", st)
		}
	}
}

func TestDemoPlansUseDemoPrefix(t *testing.T) {
	for _, p := range demoPurchaseOrderPlans() {
		if p.externalID != "" && !strings.HasPrefix(p.externalID, DemoPrefix) {
			t.Errorf("plan %s external id %q missing %s prefix", p.suffix, p.externalID, DemoPrefix)
		}
		if p.trackingNo != "" && !strings.HasPrefix(p.trackingNo, DemoPrefix) {
			t.Errorf("plan %s tracking no %q missing %s prefix", p.suffix, p.trackingNo, DemoPrefix)
		}
	}
}

func TestFullDemoSeederGuards(t *testing.T) {
	var nilSeeder *FullDemoSeeder
	if _, err := nilSeeder.Seed(context.Background()); err == nil {
		t.Error("nil seeder should error")
	}
	s := &FullDemoSeeder{AppEnv: "production"}
	if _, err := s.Seed(context.Background()); err == nil {
		t.Error("production env should be refused")
	}
	if _, err := s.Cleanup(context.Background()); err == nil {
		t.Error("production cleanup should be refused")
	}
}
