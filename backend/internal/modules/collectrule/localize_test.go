package collectrule

import (
	"errors"
	"fmt"
	"testing"
)

func TestLocalizeRuleError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("rule json exceeds size limit"), "规则 JSON 超过大小限制"},
		{ErrRuleInvalidJSON, "规则必须是 JSON 对象"},
		{ErrDomainEmpty, "域名（domain）必填"},
		{ErrMatchPatternRegexp, "matchPattern 必须是有效的正则表达式"},
		{fmt.Errorf("unknown fallback %q", "foo"), "存在未知的 fallback 键"},
		{errors.New("unsupported skus.mode"), "不支持的 skus.mode"},
		{errors.New(`url host "a.com" does not match rule domain "b.com"`), "URL 与规则域名不匹配"},
		{errors.New("url does not match rule domain or pattern"), "URL 与规则域名或匹配模式不匹配"},
		{errors.New("invalid url"), "URL 无效"},
		{errors.New("rule not found or disabled"), "规则不存在或已停用"},
		{errors.New("custom collect rule not found: please create a rule first"), "未找到自定义采集规则，请先创建规则"},
		{errors.New("collector runner unavailable"), "采集器暂不可用，请稍后重试"},
		{errors.New("collector rejected the rule"), "采集器拒绝执行该规则测试"},
		// Chinese messages pass through unchanged.
		{errors.New("状态无效：xx"), "状态无效：xx"},
		// Unmapped English messages get a generic Chinese prefix.
		{errors.New("boom"), "采集规则操作失败：boom"},
	}
	for _, c := range cases {
		if got := localizeRuleError(c.err); got != c.want {
			t.Errorf("localizeRuleError(%q) = %q, want %q", c.err, got, c.want)
		}
	}
	if got := localizeRuleError(nil); got != "" {
		t.Errorf("nil: %q", got)
	}
}
