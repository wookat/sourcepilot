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

// publishWorkerErrorMessages maps known internal worker failure messages
// (stored on tasks as user-visible error_message) to Chinese copy.
var publishWorkerErrorMessages = []struct {
	match string
	msg   string
}{
	{"empty task input", "任务缺少输入快照，无法直接重试，请重新发起刊登"},
	{"shop not available", "店铺不可用，请检查店铺授权后重试"},
	{"empty publish result", "平台未返回刊登结果，请稍后重试"},
	{"platform did not return external product id", "平台未返回商品 ID，请稍后重试"},
	{"platform product publish provider not implemented", "该平台暂不支持真实刊登，请使用本地刊登草稿"},
	{"load product:", "商品不存在或已删除，无法重试刊登"},
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

// localizePublishFailMessage converts a worker failure message to Chinese
// user-facing copy. Messages that already contain Chinese are returned
// unchanged; unmapped English messages get a generic Chinese prefix with
// the raw reason preserved for diagnostics.
func localizePublishFailMessage(msg string) string {
	raw := strings.TrimSpace(msg)
	for _, m := range publishErrorMessages {
		if strings.Contains(raw, m.match) {
			return m.msg
		}
	}
	for _, m := range publishWorkerErrorMessages {
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
		return "刊登失败，请稍后重试"
	}
	return "刊登失败：" + raw
}
