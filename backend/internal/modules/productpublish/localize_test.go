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
