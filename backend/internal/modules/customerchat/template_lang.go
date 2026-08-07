package customerchat

import (
	"encoding/json"
	"strings"
)

// 多语言模板语言表（可扩展）：语言码采用 BCP-47 风格短码。新增语言只需
// 在此追加一项，前端 admin/src/services/customer.ts TEMPLATE_LANGUAGES 同步补中文标签。
const TemplateDefaultLanguage = "zh-CN"

// TemplateLanguages lists all selectable template languages.
var TemplateLanguages = []string{
	"zh-CN", "en", "es", "pt", "fr", "de", "it", "ru", "ja", "ko",
	"th", "vi", "id", "ms", "ar",
}

// IsValidTemplateLanguage reports whether lang is in the language table.
func IsValidTemplateLanguage(lang string) bool {
	for _, l := range TemplateLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

// NormalizeTemplateLanguage maps loose inputs (case / region variants) onto
// the language table; returns "" when unrecognized.
func NormalizeTemplateLanguage(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if IsValidTemplateLanguage(s) {
		return s
	}
	lower := strings.ToLower(strings.ReplaceAll(s, "_", "-"))
	switch {
	case strings.HasPrefix(lower, "zh"):
		return "zh-CN"
	}
	base := lower
	if i := strings.Index(lower, "-"); i > 0 {
		base = lower[:i]
	}
	if IsValidTemplateLanguage(base) {
		return base
	}
	return ""
}

// Draft language sources（草稿目标语言来源，前端据此展示推断/回退标注）.
const (
	BuyerMsgLangSourceOrderCountry = "order_country" // 订单收货地国家推断
	BuyerMsgLangSourceShopLanguage = "shop_language" // 店铺默认语言配置
	BuyerMsgLangSourcePlatform     = "platform"      // 店铺平台推断
	BuyerMsgLangSourceFallback     = "fallback"      // 无法推断，回退模板默认语言
	BuyerMsgLangSourceNoVariant    = "no_variant"    // 推断成功但模板缺该语言变体，回退默认语言
	BuyerMsgLangSourceManual       = "manual"        // 工作台人工切换语言重新生成
)

// countryLanguage maps ISO 3166-1 alpha-2 country codes (upper-case) onto the
// language table. Best-effort: only unambiguous mappings are listed.
var countryLanguage = map[string]string{
	"CN": "zh-CN", "TW": "zh-CN", "HK": "zh-CN", "MO": "zh-CN", "SG": "zh-CN",
	"US": "en", "GB": "en", "UK": "en", "AU": "en", "CA": "en", "NZ": "en",
	"IE": "en", "PH": "en", "IN": "en", "ZA": "en", "NG": "en",
	"ES": "es", "MX": "es", "AR": "es", "CL": "es", "CO": "es", "PE": "es",
	"VE": "es", "EC": "es", "UY": "es", "BO": "es", "PY": "es", "GT": "es",
	"BR": "pt", "PT": "pt", "AO": "pt", "MZ": "pt",
	"FR": "fr", "BE": "fr",
	"DE": "de", "AT": "de", "CH": "de",
	"IT": "it",
	"RU": "ru", "BY": "ru", "KZ": "ru",
	"JP": "ja",
	"KR": "ko",
	"TH": "th",
	"VN": "vi",
	"ID": "id",
	"MY": "ms", "BN": "ms",
	"SA": "ar", "AE": "ar", "EG": "ar", "KW": "ar", "QA": "ar", "OM": "ar",
	"BH": "ar", "JO": "ar", "IQ": "ar", "MA": "ar", "DZ": "ar", "TN": "ar",
}

// platformLanguage maps shop platforms onto a default buyer language. Only
// platforms whose buyer base is unambiguous are listed; marketplaces that span
// many locales (shopee / tiktok / amazon / lazada…) rely on shop DefaultLanguage
// or order country instead.
var platformLanguage = map[string]string{
	"douyin": "zh-CN", "taobao": "zh-CN", "tmall": "zh-CN", "pdd": "zh-CN",
	"1688": "zh-CN", "jd": "zh-CN", "xhs": "zh-CN", "kuaishou": "zh-CN",
	"mercadolibre": "es",
}

// extractOrderCountry pulls a country / countryCode string from order RawData
// (top level or nested receiver / address / shippingAddress objects).
func extractOrderCountry(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	keys := []string{"countryCode", "country", "receiverCountry"}
	pick := func(obj map[string]any) string {
		for _, k := range keys {
			if v, ok := obj[k].(string); ok {
				if t := strings.TrimSpace(v); t != "" {
					return t
				}
			}
		}
		return ""
	}
	if v := pick(m); v != "" {
		return v
	}
	for _, nested := range []string{"receiver", "address", "shippingAddress"} {
		if obj, ok := m[nested].(map[string]any); ok {
			if v := pick(obj); v != "" {
				return v
			}
		}
	}
	return ""
}

// countryToLanguage maps a raw country value (alpha-2 code or a few common
// names) to a template language; "" when unknown.
func countryToLanguage(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if lang, ok := countryLanguage[strings.ToUpper(s)]; ok {
		return lang
	}
	switch s {
	case "中国", "中国大陆", "中国台湾", "中国香港":
		return "zh-CN"
	}
	switch strings.ToLower(s) {
	case "united states", "usa", "united kingdom", "australia", "canada":
		return "en"
	case "brazil", "brasil", "portugal":
		return "pt"
	case "spain", "mexico", "españa", "méxico":
		return "es"
	}
	return ""
}
