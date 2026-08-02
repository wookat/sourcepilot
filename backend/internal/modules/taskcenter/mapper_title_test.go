package taskcenter

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
)

// 任务标题应展示平台中文名，不出现裸平台编码（回归：AI 工作台/任务中心标题裸 douyin_shop）。
func TestMapOrderSyncTaskTitleUsesPlatformLabel(t *testing.T) {
	row := &ordersync.OrderSyncTask{
		ShopID:   uuid.New(),
		Platform: "douyin_shop",
		Status:   "failed",
	}
	dto := mapOrderSyncTask(row, map[uuid.UUID]string{}, markSet{}, time.Now())
	if strings.Contains(dto.Title, "douyin_shop") {
		t.Fatalf("title should not contain raw platform code, got %q", dto.Title)
	}
	if !strings.Contains(dto.Title, "抖店") {
		t.Fatalf("title should contain platform label, got %q", dto.Title)
	}
	if dto.Platform != "douyin_shop" {
		t.Fatalf("platform field should keep raw code, got %q", dto.Platform)
	}
}
