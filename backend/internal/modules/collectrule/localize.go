package collectrule

import "strings"

// ruleErrorMessages maps known English rule error fragments to user-facing
// Chinese copy. Matching is first-hit substring, so more specific fragments
// must come before generic ones.
var ruleErrorMessages = []struct {
	match string
	msg   string
}{
	{"rule json exceeds size limit", "规则 JSON 超过大小限制"},
	{"rule must be a JSON object", "规则必须是 JSON 对象"},
	{"rule schema validation failed", "规则结构校验失败"},
	{"domain is required", "域名（domain）必填"},
	{"matchPattern must be a valid regexp", "matchPattern 必须是有效的正则表达式"},
	{"unknown fallback", "存在未知的 fallback 键"},
	{"unsupported skus.mode", "不支持的 skus.mode"},
	{"unsupported attributes.mode", "不支持的 attributes.mode"},
	{"too many selectors", "选择器数量超过上限"},
	{"forbidden selector content", "选择器包含禁止的内容"},
	{"fallback too long", "fallback 文本过长"},
	{"unsupported attr", "存在不支持的 attr"},
	{"limit out of range", "limit 超出允许范围"},
	{"selector longer than", "选择器长度超过上限"},
	{"does not match rule domain or pattern", "URL 与规则域名或匹配模式不匹配"},
	{"does not match rule domain", "URL 与规则域名不匹配"},
	{"does not match rule pattern", "URL 与规则匹配模式不匹配"},
	{"url must start with", "URL 必须以 http:// 或 https:// 开头"},
	{"url host required", "URL 缺少主机名"},
	{"invalid url", "URL 无效"},
	{"unsupported scheme", "URL 协议不支持（仅支持 http/https）"},
	{"missing host", "URL 缺少主机名"},
	{"rule not found or disabled", "规则不存在或已停用"},
	{"rule source mismatch", "规则来源与采集来源不一致"},
	{"custom collect rule not found", "未找到自定义采集规则，请先创建规则"},
	{"name is required", "规则名称必填"},
	{"name cannot be empty", "规则名称不能为空"},
	{"invalid status", "状态无效"},
	{"rule is required", "规则内容必填"},
	{"rule cannot be empty", "规则内容不能为空"},
	{"collector runner unavailable", "采集器暂不可用，请稍后重试"},
	{"unsupported rule source", "不支持的规则来源"},
	{"invalid profileId", "profileId 无效"},
	{"collector rejected", "采集器拒绝执行该规则测试"},
}

// localizeRuleError converts a rule service error to Chinese user-facing
// copy. Messages already containing Chinese pass through unchanged; unmapped
// English messages get a generic Chinese prefix with the raw reason
// preserved for diagnostics.
func localizeRuleError(err error) string {
	if err == nil {
		return ""
	}
	raw := strings.TrimSpace(err.Error())
	for _, m := range ruleErrorMessages {
		if strings.Contains(raw, m.match) {
			return m.msg
		}
	}
	for _, r := range raw {
		if r > 0x2E7F { // contains CJK: already user-facing Chinese copy
			return raw
		}
	}
	if raw == "" {
		return "采集规则操作失败，请稍后重试"
	}
	return "采集规则操作失败：" + raw
}
