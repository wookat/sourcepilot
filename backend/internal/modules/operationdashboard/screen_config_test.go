package operationdashboard

import (
	"testing"
)

func TestDefaultScreenCards(t *testing.T) {
	cards := defaultScreenCards()
	if len(cards) != len(screenCardDefaultOrder) {
		t.Fatalf("expected %d cards, got %d", len(screenCardDefaultOrder), len(cards))
	}
	for i, c := range cards {
		if c.Key != screenCardDefaultOrder[i] {
			t.Fatalf("card %d: expected %s, got %s", i, screenCardDefaultOrder[i], c.Key)
		}
		if !c.Enabled {
			t.Fatalf("default card %s must be enabled", c.Key)
		}
		if c.Title == "" {
			t.Fatalf("card %s missing title", c.Key)
		}
	}
}

func TestNormalizeScreenCards(t *testing.T) {
	t.Run("custom order kept, missing appended enabled", func(t *testing.T) {
		out, err := normalizeScreenCards([]storedScreenCard{
			{Key: CardTrend, Enabled: true},
			{Key: CardKPISales, Enabled: false},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != len(screenCardDefaultOrder) {
			t.Fatalf("expected %d cards, got %d", len(screenCardDefaultOrder), len(out))
		}
		if out[0].Key != CardTrend || !out[0].Enabled {
			t.Fatalf("first card should be enabled trend, got %+v", out[0])
		}
		if out[1].Key != CardKPISales || out[1].Enabled {
			t.Fatalf("second card should be disabled kpi_sales, got %+v", out[1])
		}
		for _, c := range out[2:] {
			if !c.Enabled {
				t.Fatalf("appended card %s should default to enabled", c.Key)
			}
		}
	})
	t.Run("unknown key rejected", func(t *testing.T) {
		if _, err := normalizeScreenCards([]storedScreenCard{{Key: "nope", Enabled: true}}); err == nil {
			t.Fatal("expected error for unknown key")
		}
	})
	t.Run("duplicate key rejected", func(t *testing.T) {
		if _, err := normalizeScreenCards([]storedScreenCard{
			{Key: CardTodos, Enabled: true}, {Key: CardTodos, Enabled: false},
		}); err == nil {
			t.Fatal("expected error for duplicate key")
		}
	})
}

func TestParseScreenCardsJSON(t *testing.T) {
	for _, raw := range []string{"", "not json", "[]", `[{"key":"bad","enabled":true}]`} {
		out := parseScreenCardsJSON(raw)
		if len(out) != len(screenCardDefaultOrder) || out[0].Key != screenCardDefaultOrder[0] || !out[0].Enabled {
			t.Fatalf("raw %q should fall back to defaults, got %+v", raw, out)
		}
	}
	out := parseScreenCardsJSON(`[{"key":"funnel","enabled":false},{"key":"kpi_orders","enabled":true}]`)
	if out[0].Key != CardFunnel || out[0].Enabled {
		t.Fatalf("expected disabled funnel first, got %+v", out[0])
	}
	if out[1].Key != CardKPIOrders || !out[1].Enabled {
		t.Fatalf("expected enabled kpi_orders second, got %+v", out[1])
	}
}

func TestEnabledCardSet(t *testing.T) {
	set := enabledCardSet([]ScreenCardDTO{
		{Key: CardTodos, Enabled: true},
		{Key: CardFunnel, Enabled: false},
	})
	if !set[CardTodos] || set[CardFunnel] {
		t.Fatalf("unexpected set: %+v", set)
	}
}
