package productpublish

import "strings"

// publishErrorMessages maps known English publish payload validation
// messages (from services and platform providers) to user-facing Chinese.
var publishErrorMessages = []struct {
	match string
	msg   string
}{
	{"product main image required for publish", "商品缺少主图，无法刊登，请先在图片管理中设置商品主图"},
	{"douyin main image required", "抖店商品主图未上传，请先上传主图后重试"},
	{"targets required", "请至少选择一个刊登目标"},
	{"invalid shopId", "店铺 ID 无效，请重新选择店铺"},
	{"invalid json body", "请求内容格式不正确"},
}

// localizePublishError converts publish validation errors to user-facing
// Chinese copy, falling back to the original message when unmapped.
func localizePublishError(err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	for _, m := range publishErrorMessages {
		if strings.Contains(raw, m.match) {
			return m.msg
		}
	}
	return raw
}
