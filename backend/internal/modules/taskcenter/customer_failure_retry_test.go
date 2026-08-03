package taskcenter

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 失败中心口径：客服异常仅「AI 回复生成失败」可通过重试（重新生成建议）修复；
// 发送失败/权限类必须回会话页人工处理，不得标记为可重试。
func TestMapCustomerFailureEventRetryableByCategory(t *testing.T) {
	now := time.Now()
	base := customerchat.CustomerFailureEvent{
		ConversationID: uuid.New(),
		Platform:       "douyin",
		Status:         customerchat.FailureEventStatusOpen,
	}

	gen := base
	gen.Category = customerchat.FailureCategoryReplyGenerateFailed
	if dto := mapCustomerFailureEvent(&gen, markSet{}, now); !dto.Retryable {
		t.Fatal("reply_generate_failed should be retryable")
	}

	for _, cat := range []string{
		customerchat.FailureCategoryReplySendFailed,
		customerchat.FailureCategoryReplyPermissionDenied,
		customerchat.FailureCategoryPlatformNotAuthorized,
	} {
		ev := base
		ev.Category = cat
		if dto := mapCustomerFailureEvent(&ev, markSet{}, now); dto.Retryable {
			t.Fatalf("%s should not be retryable", cat)
		}
	}
}

// 回归：R84 前 RetryFailure 对 customer_failure 返回「unknown task type」。
// 现在 unifiedOne / 重试分支都必须识别该类型。
func TestRetryFailureRecognizesCustomerFailure(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "tc.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&customerchat.CustomerFailureEvent{}, &TaskFailureMark{}); err != nil {
		t.Fatal(err)
	}
	ev := &customerchat.CustomerFailureEvent{
		ConversationID: uuid.New(),
		Category:       customerchat.FailureCategoryReplyGenerateFailed,
		Status:         customerchat.FailureEventStatusOpen,
	}
	if err := db.Create(ev).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/retry", nil)

	svc := &Service{DB: db}
	retryErr := svc.RetryFailure(c, TaskTypeCustomerFailure, ev.ID)
	if retryErr == nil {
		t.Fatal("expected error when CustomerChat service is not wired")
	}
	if strings.Contains(retryErr.Error(), "unknown task type") ||
		strings.Contains(retryErr.Error(), "unsupported task type") {
		t.Fatalf("customer_failure must be recognized by retry dispatch, got %q", retryErr.Error())
	}
}
