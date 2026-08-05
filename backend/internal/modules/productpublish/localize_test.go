package productpublish

import (
	"errors"
	"fmt"
	"testing"
)

func TestLocalizePublishError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"main image", errors.New("product main image required for publish"), "商品缺少主图，无法刊登，请先在图片管理中设置商品主图"},
		{"main image wrapped", fmt.Errorf("build draft: %w", errors.New("product main image required for publish")), "商品缺少主图，无法刊登，请先在图片管理中设置商品主图"},
		{"targets required", errors.New("targets required"), "请至少选择一个刊登目标"},
		{"invalid shopId", errors.New("invalid shopId"), "店铺 ID 无效，请重新选择店铺"},
		{"douyin main image", errors.New("douyin main image required"), "抖店商品主图未上传，请先上传主图后重试"},
		{"unknown passthrough", errors.New("店铺未授权"), "店铺未授权"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := localizePublishError(tc.err); got != tc.want {
				t.Fatalf("localizePublishError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestLocalizePublishFailMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want string
	}{
		{"empty task input", "empty task input", "任务缺少输入快照，无法直接重试，请重新发起刊登"},
		{"shop not available", "shop not available", "店铺不可用，请检查店铺授权后重试"},
		{"load product", "load product: record not found", "商品不存在或已删除，无法重试刊登"},
		{"not implemented", "platform product publish provider not implemented", "该平台暂不支持真实刊登，请使用本地刊登草稿"},
		{"main image via shared map", "product main image required for publish", "商品缺少主图，无法刊登，请先在图片管理中设置商品主图"},
		{"chinese passthrough", "店铺未授权", "店铺未授权"},
		{"empty fallback", "", "刊登失败，请稍后重试"},
		{"unknown english gets chinese prefix", "some upstream failure", "刊登失败：some upstream failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := localizePublishFailMessage(tc.msg); got != tc.want {
				t.Fatalf("localizePublishFailMessage(%q) = %q, want %q", tc.msg, got, tc.want)
			}
		})
	}
}
