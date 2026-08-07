package customerchat

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func TestNormalizeTemplateLanguage(t *testing.T) {
	cases := map[string]string{
		"zh-CN": "zh-CN", "zh_TW": "zh-CN", "ZH": "zh-CN",
		"en": "en", "en-US": "en", "EN_GB": "en",
		"es": "es", "pt-BR": "pt",
		"": "", "  ": "", "klingon": "",
	}
	for in, want := range cases {
		if got := NormalizeTemplateLanguage(in); got != want {
			t.Errorf("NormalizeTemplateLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCountryToLanguage(t *testing.T) {
	cases := map[string]string{
		"US": "en", "us": "en", "BR": "pt", "ES": "es", "CN": "zh-CN",
		"JP": "ja", "Brazil": "pt", "中国": "zh-CN", "": "", "XX": "",
	}
	for in, want := range cases {
		if got := countryToLanguage(in); got != want {
			t.Errorf("countryToLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractOrderCountry(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"countryCode":"US"}`, "US"},
		{`{"receiver":{"countryCode":"BR"}}`, "BR"},
		{`{"address":{"country":"ES"}}`, "ES"},
		{`{"shippingAddress":{"receiverCountry":"JP"}}`, "JP"},
		{`{"other":"x"}`, ""},
		{``, ""},
		{`not-json`, ""},
	}
	for _, c := range cases {
		if got := extractOrderCountry(json.RawMessage(c.raw)); got != c.want {
			t.Errorf("extractOrderCountry(%s) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func mustRawData(t *testing.T, v any) datatypes.JSON {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return datatypes.JSON(b)
}

func TestResolveBuyerMsgLanguagePriority(t *testing.T) {
	// ① order country wins
	o := &order.Order{RawData: mustRawData(t, map[string]any{"receiver": map[string]any{"countryCode": "BR"}})}
	octx := buyerMsgOrderCtx{shopLanguage: "en", shopPlatform: "mercadolibre"}
	if lang, src := resolveBuyerMsgLanguage(o, octx, "zh-CN"); lang != "pt" || src != BuyerMsgLangSourceOrderCountry {
		t.Fatalf("order country: %s/%s", lang, src)
	}
	// ② shop language
	o = &order.Order{}
	if lang, src := resolveBuyerMsgLanguage(o, octx, "zh-CN"); lang != "en" || src != BuyerMsgLangSourceShopLanguage {
		t.Fatalf("shop language: %s/%s", lang, src)
	}
	// ③ platform
	octx = buyerMsgOrderCtx{shopPlatform: "taobao"}
	if lang, src := resolveBuyerMsgLanguage(o, octx, "en"); lang != "zh-CN" || src != BuyerMsgLangSourcePlatform {
		t.Fatalf("platform: %s/%s", lang, src)
	}
	// ③ order platform used when shop platform empty
	o = &order.Order{Platform: "mercadolibre"}
	if lang, src := resolveBuyerMsgLanguage(o, buyerMsgOrderCtx{}, "zh-CN"); lang != "es" || src != BuyerMsgLangSourcePlatform {
		t.Fatalf("order platform: %s/%s", lang, src)
	}
	// ④ fallback（负样本：无收货地、无店铺语言、平台不明确）
	o = &order.Order{Platform: "shopee"}
	if lang, src := resolveBuyerMsgLanguage(o, buyerMsgOrderCtx{}, "zh-CN"); lang != "zh-CN" || src != BuyerMsgLangSourceFallback {
		t.Fatalf("fallback: %s/%s", lang, src)
	}
}

func TestBuyerMsgTemplateContent(t *testing.T) {
	tpl := CustomerReplyTemplate{Content: "默认内容", DefaultLanguage: "zh-CN"}
	variants := map[string]string{"en": "english content"}

	// default language → base content
	if c, l, s := buyerMsgTemplateContent(tpl, variants, "zh-CN", BuyerMsgLangSourcePlatform); c != "默认内容" || l != "zh-CN" || s != BuyerMsgLangSourcePlatform {
		t.Fatalf("default: %s/%s/%s", c, l, s)
	}
	// variant hit
	if c, l, s := buyerMsgTemplateContent(tpl, variants, "en", BuyerMsgLangSourceOrderCountry); c != "english content" || l != "en" || s != BuyerMsgLangSourceOrderCountry {
		t.Fatalf("variant: %s/%s/%s", c, l, s)
	}
	// variant missing → fallback to default with no_variant（负样本）
	if c, l, s := buyerMsgTemplateContent(tpl, variants, "es", BuyerMsgLangSourceOrderCountry); c != "默认内容" || l != "zh-CN" || s != BuyerMsgLangSourceNoVariant {
		t.Fatalf("no variant: %s/%s/%s", c, l, s)
	}
	// legacy template without DefaultLanguage behaves as zh-CN
	legacy := CustomerReplyTemplate{Content: "旧内容"}
	if c, l, _ := buyerMsgTemplateContent(legacy, nil, "zh-CN", BuyerMsgLangSourceFallback); c != "旧内容" || l != "zh-CN" {
		t.Fatalf("legacy: %s/%s", c, l)
	}
}

func TestTemplateVariantsCRUDAndValidation(t *testing.T) {
	c, svc := newTemplateTestCtx(t, 7)

	// invalid variant language rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "x", Content: "y",
		Variants: &[]TemplateVariantRow{{Language: "klingon", Content: "z"}},
	}, nil); err == nil {
		t.Fatal("invalid variant language must be rejected")
	}
	// variant equal to default language rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "x", Content: "y", DefaultLanguage: "zh-CN",
		Variants: &[]TemplateVariantRow{{Language: "zh-CN", Content: "z"}},
	}, nil); err == nil {
		t.Fatal("variant equal to default language must be rejected")
	}
	// duplicate variant languages rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "x", Content: "y",
		Variants: &[]TemplateVariantRow{{Language: "en", Content: "a"}, {Language: "en", Content: "b"}},
	}, nil); err == nil {
		t.Fatal("duplicate variant languages must be rejected")
	}
	// empty variant content rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "x", Content: "y",
		Variants: &[]TemplateVariantRow{{Language: "en", Content: "  "}},
	}, nil); err == nil {
		t.Fatal("empty variant content must be rejected")
	}
	// invalid default language rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "x", Content: "y", DefaultLanguage: "klingon",
	}, nil); err == nil {
		t.Fatal("invalid default language must be rejected")
	}

	row, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "多语言-发货", Content: "订单{订单号}已发货",
		DefaultLanguage: "zh-CN",
		Variants: &[]TemplateVariantRow{
			{Language: "en", Content: "Order {订单号} shipped"},
			{Language: "es", Content: "Pedido {订单号} enviado"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("CreateTemplate with variants: %v", err)
	}
	if row.DefaultLanguage != "zh-CN" || len(row.Variants) != 2 {
		t.Fatalf("create result: %+v", row)
	}
	var stored []CustomerReplyTemplateVariant
	if err := svc.DB.Where("template_id = ?", row.ID).Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].TenantID != 7 {
		t.Fatalf("stored variants: %+v", stored)
	}

	// update replaces the full variant set
	upd, err := svc.UpdateTemplate(c, row.ID, TemplateUpsertBody{
		Variants: &[]TemplateVariantRow{{Language: "pt", Content: "Pedido {订单号} enviado"}},
	}, nil)
	if err != nil {
		t.Fatalf("UpdateTemplate variants: %v", err)
	}
	if len(upd.Variants) != 1 || upd.Variants[0].Language != "pt" {
		t.Fatalf("update result: %+v", upd.Variants)
	}

	// list returns variants
	rows, err := svc.ListTemplates(c, TemplateListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0].Variants) != 1 {
		t.Fatalf("list variants: %+v", rows)
	}

	// delete removes variants too
	if err := svc.DeleteTemplate(c, row.ID, nil); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := svc.DB.Model(&CustomerReplyTemplateVariant{}).Where("template_id = ?", row.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("variants must be deleted with template, got %d", n)
	}
}

func TestRegenerateBuyerMsgDraft(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)

	tplRow, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "多语言-发货", Content: "订单{订单号}已发货",
		DefaultLanguage: "zh-CN",
		Variants:        &[]TemplateVariantRow{{Language: "en", Content: "Order {订单号} shipped"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := order.Order{TenantID: 7, OrderNo: "SO-1", CustomerName: "张三", Platform: "shopee", Status: order.StatusShipped}
	if err := svc.DB.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	draft := BuyerMessageDraft{
		TenantID: 7, OrderID: o.ID, Node: BuyerMsgNodeShipped,
		RuleID: uuid.New(), TemplateID: tplRow.ID, TemplateName: tplRow.Name,
		OrderNo: o.OrderNo, CustomerName: o.CustomerName,
		Content: "订单SO-1已发货", Language: "zh-CN", LangSource: BuyerMsgLangSourceFallback,
		Status: BuyerMsgDraftPending,
	}
	if err := svc.DB.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}

	// invalid language rejected
	if _, err := svc.RegenerateBuyerMsgDraft(c, draft.ID, "klingon", nil); err == nil {
		t.Fatal("invalid language must be rejected")
	}
	// missing variant rejected（负样本）
	if _, err := svc.RegenerateBuyerMsgDraft(c, draft.ID, "es", nil); err == nil {
		t.Fatal("missing variant must be rejected")
	}
	// switch to en variant
	out, err := svc.RegenerateBuyerMsgDraft(c, draft.ID, "en", nil)
	if err != nil {
		t.Fatalf("RegenerateBuyerMsgDraft: %v", err)
	}
	if out.Content != "Order SO-1 shipped" || out.Language != "en" || out.LangSource != BuyerMsgLangSourceManual {
		t.Fatalf("regenerated: %+v", out)
	}
	// switch back to default language
	out, err = svc.RegenerateBuyerMsgDraft(c, draft.ID, "zh-CN", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Content != "订单SO-1已发货" || out.Language != "zh-CN" {
		t.Fatalf("back to default: %+v", out)
	}

	// cross-tenant → not found
	c8 := buyerMsgCtx(t, 8)
	if _, err := svc.RegenerateBuyerMsgDraft(c8, draft.ID, "en", nil); err != ErrBuyerMsgDraftNotFound {
		t.Fatalf("cross-tenant: %v", err)
	}

	// non-pending rejected
	if err := svc.DB.Model(&BuyerMessageDraft{}).Where("id = ?", draft.ID).
		Update("status", BuyerMsgDraftSent).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegenerateBuyerMsgDraft(c, draft.ID, "en", nil); err == nil {
		t.Fatal("non-pending draft must be rejected")
	}
}

func TestGenerateBuyerMsgDraftsPicksLanguageVariant(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)

	tplRow, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "多语言-发货", Content: "订单{订单号}已发货",
		DefaultLanguage: "zh-CN",
		Variants:        &[]TemplateVariantRow{{Language: "pt", Content: "Pedido {订单号} enviado"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	backfill := true
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "发货通知", Node: BuyerMsgNodeShipped, TemplateID: tplRow.ID.String(), Backfill: &backfill,
	}, nil); err != nil {
		t.Fatal(err)
	}

	// 正样本：巴西收货地 → pt 变体
	oBR := order.Order{
		TenantID: 7, OrderNo: "SO-BR", CustomerName: "Ana", Platform: "shopee",
		Status:  order.StatusShipped,
		RawData: mustRawData(t, map[string]any{"receiver": map[string]any{"countryCode": "BR"}}),
	}
	// 正样本：美国收货地但缺 en 变体 → 回退默认并标注 no_variant
	oUS := order.Order{
		TenantID: 7, OrderNo: "SO-US", CustomerName: "Bob", Platform: "shopee",
		Status:  order.StatusShipped,
		RawData: mustRawData(t, map[string]any{"countryCode": "US"}),
	}
	// 负样本：无法推断 → 回退默认语言并标注 fallback
	oNone := order.Order{
		TenantID: 7, OrderNo: "SO-NONE", CustomerName: "王五", Platform: "shopee",
		Status: order.StatusShipped,
	}
	for _, o := range []*order.Order{&oBR, &oUS, &oNone} {
		if err := svc.DB.Create(o).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.GenerateBuyerMsgDrafts(c.Request.Context(), 7); err != nil {
		t.Fatal(err)
	}

	get := func(orderNo string) BuyerMessageDraft {
		var d BuyerMessageDraft
		if err := svc.DB.Where("order_no = ?", orderNo).First(&d).Error; err != nil {
			t.Fatalf("draft for %s: %v", orderNo, err)
		}
		return d
	}
	if d := get("SO-BR"); d.Language != "pt" || d.LangSource != BuyerMsgLangSourceOrderCountry || d.Content != "Pedido SO-BR enviado" {
		t.Fatalf("BR draft: %+v", d)
	}
	if d := get("SO-US"); d.Language != "zh-CN" || d.LangSource != BuyerMsgLangSourceNoVariant {
		t.Fatalf("US draft: %+v", d)
	}
	if d := get("SO-NONE"); d.Language != "zh-CN" || d.LangSource != BuyerMsgLangSourceFallback {
		t.Fatalf("NONE draft: %+v", d)
	}
}
