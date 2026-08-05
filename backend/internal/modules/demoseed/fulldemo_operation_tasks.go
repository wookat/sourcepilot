package demoseed

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

// operationTaskPlan describes one demo operation task walked through the real
// task state machine (see operationtask/state_machine.go). statusChain lists
// statuses after suggested, applied in order.
type operationTaskPlan struct {
	suffix      string
	title       string
	summary     string
	sourceType  string
	taskType    string
	priority    string
	statusChain []string
	// withDraft creates one platform draft; draftStatus is its final status.
	withDraft   bool
	draftStatus string
	// approval decision recorded against the draft ("" = none).
	approvalDecision string
	// withAttempt creates one execution attempt with attemptStatus; failed
	// attempts also append one execution error with errRetryable.
	withAttempt   bool
	attemptStatus string
	errCode       string
	errCategory   string
	errMessage    string
	errRetryable  bool
}

func demoOperationTaskPlans() []operationTaskPlan {
	return []operationTaskPlan{
		{suffix: "OT-SUGGEST", title: "DEMO-AI建议 优化商品主图与卖点文案",
			summary:    "DEMO- 演示：AI 建议任务（待生成草稿）",
			sourceType: operationtask.OperationTaskSourceAISuggestion,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityHigh},
		{suffix: "OT-REVIEW", title: "DEMO-AI建议 标题本地化改写待审核",
			summary:    "DEMO- 演示：草稿已生成，等待人工审核",
			sourceType: operationtask.OperationTaskSourceAISuggestion,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityNormal,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusPendingReview},
		{suffix: "OT-REVIEW-2", title: "DEMO-AI建议 商品卖点描述重写待审核",
			summary:    "DEMO- 演示：草稿已生成，等待人工审核（批量审批样本）",
			sourceType: operationtask.OperationTaskSourceAISuggestion,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityHigh,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusPendingReview},
		{suffix: "OT-REJECTED", title: "DEMO-AI建议 描述重写（已驳回）",
			summary:    "DEMO- 演示：审核驳回样本",
			sourceType: operationtask.OperationTaskSourceAISuggestion,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityLow,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview,
				operationtask.OperationTaskStatusRejected},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusPendingReview,
			approvalDecision: operationtask.ApprovalDecisionRejected},
		{suffix: "OT-DONE", title: "DEMO-运营任务 商品草稿本地写入（已完成）",
			summary:    "DEMO- 演示：审核通过并执行成功",
			sourceType: operationtask.OperationTaskSourceManual,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityNormal,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview,
				operationtask.OperationTaskStatusApproved,
				operationtask.OperationTaskStatusExecutionQueued,
				operationtask.OperationTaskStatusExecuting,
				operationtask.OperationTaskStatusDraftWritten},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusWritten,
			approvalDecision: operationtask.ApprovalDecisionApproved,
			withAttempt:      true, attemptStatus: operationtask.ExecutionAttemptStatusSucceeded},
		{suffix: "OT-FAIL-RETRY", title: "DEMO-运营任务 执行失败（可重试）",
			summary:    "DEMO- 演示：执行超时失败，可人工重试",
			sourceType: operationtask.OperationTaskSourceRuleEngine,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityUrgent,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview,
				operationtask.OperationTaskStatusApproved,
				operationtask.OperationTaskStatusExecutionQueued,
				operationtask.OperationTaskStatusExecuting,
				operationtask.OperationTaskStatusExecutionFailed},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusApproved,
			approvalDecision: operationtask.ApprovalDecisionApproved,
			withAttempt:      true, attemptStatus: operationtask.ExecutionAttemptStatusFailed,
			errCode:      "context_deadline_exceeded",
			errCategory:  operationtask.ExecutionErrorCategoryProviderTimeout,
			errMessage:   "DEMO- 演示：草稿写入超时，可重试",
			errRetryable: true},
		{suffix: "OT-FAIL-FINAL", title: "DEMO-运营任务 执行失败（不可重试）",
			summary:    "DEMO- 演示：适配器模式不支持，不可重试",
			sourceType: operationtask.OperationTaskSourceRuleEngine,
			taskType:   operationtask.OperationTaskTypeProductContent,
			priority:   operationtask.OperationTaskPriorityHigh,
			statusChain: []string{operationtask.OperationTaskStatusDraftPreparing,
				operationtask.OperationTaskStatusPendingReview,
				operationtask.OperationTaskStatusApproved,
				operationtask.OperationTaskStatusExecutionQueued,
				operationtask.OperationTaskStatusExecuting,
				operationtask.OperationTaskStatusExecutionFailed},
			withDraft: true, draftStatus: operationtask.PlatformDraftStatusFailed,
			approvalDecision: operationtask.ApprovalDecisionApproved,
			withAttempt:      true, attemptStatus: operationtask.ExecutionAttemptStatusFailed,
			errCode:      "unsupported_adapter_mode",
			errCategory:  operationtask.ExecutionErrorCategoryAdapterUnavailable,
			errMessage:   "DEMO- 演示：当前平台适配器模式不支持该写入",
			errRetryable: false},
	}
}

func validateOperationTaskChain(plan operationTaskPlan) error {
	sm := operationtask.NewTaskStateMachine()
	from := operationtask.OperationTaskStatusSuggested
	for _, to := range plan.statusChain {
		if !sm.CanTransition(from, to) {
			return fmt.Errorf("demoseed: illegal operation task transition %s -> %s (plan %s)", from, to, plan.suffix)
		}
		from = to
	}
	return nil
}

// eventTypeForTransition maps a task status transition to its audit event type.
func eventTypeForTransition(to string) string {
	switch to {
	case operationtask.OperationTaskStatusDraftPreparing:
		return operationtask.OperationTaskEventTypeDraftGenerated
	case operationtask.OperationTaskStatusPendingReview:
		return operationtask.OperationTaskEventTypeReviewRequested
	case operationtask.OperationTaskStatusApproved:
		return operationtask.OperationTaskEventTypeApproved
	case operationtask.OperationTaskStatusRejected:
		return operationtask.OperationTaskEventTypeRejected
	case operationtask.OperationTaskStatusExecutionQueued:
		return operationtask.OperationTaskEventTypeExecutionQueued
	case operationtask.OperationTaskStatusExecuting:
		return operationtask.OperationTaskEventTypeExecutionStarted
	case operationtask.OperationTaskStatusDraftWritten:
		return operationtask.OperationTaskEventTypeDraftWritten
	case operationtask.OperationTaskStatusExecutionFailed:
		return operationtask.OperationTaskEventTypeExecutionFailed
	case operationtask.OperationTaskStatusCancelled:
		return operationtask.OperationTaskEventTypeCancelled
	default:
		return operationtask.OperationTaskEventTypeTaskCreated
	}
}

// seedOperationTasks inserts DEMO- operation tasks covering the review /
// execution lifecycle: an AI suggestion, a pending review draft, a rejected
// draft, a successful write, plus retryable and non-retryable failures with
// full draft / approval / attempt / error / event audit trails.
func (s *FullDemoSeeder) seedOperationTasks(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop, products []product.Product) error {
	if !tx.Migrator().HasTable("operation_tasks") {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }
	reviewerID := uuid.New()

	for i, plan := range demoOperationTaskPlans() {
		if err := validateOperationTaskChain(plan); err != nil {
			return err
		}
		payload := mustJSON(map[string]any{
			"seedPrefix": DemoPrefix,
			"productId":  products[i%len(products)].ID.String(),
			"note":       plan.summary,
		})
		payloadHash, err := operationtask.ComputePayloadHash(payload)
		if err != nil {
			return fmt.Errorf("demoseed: operation task payload hash: %w", err)
		}
		idem := fmt.Sprintf("DEMO-%s", plan.suffix)
		status := operationtask.OperationTaskStatusSuggested
		if len(plan.statusChain) > 0 {
			status = plan.statusChain[len(plan.statusChain)-1]
		}
		shopID := shops[1].ID
		task := operationtask.OperationTask{
			TenantID:        s.TenantID,
			ShopID:          &shopID,
			SourceType:      plan.sourceType,
			SourceReference: products[i%len(products)].ID.String(),
			TaskType:        plan.taskType,
			Platform:        operationtask.PlatformLocal,
			Title:           plan.title,
			Summary:         plan.summary,
			Payload:         payload,
			Status:          status,
			Priority:        plan.priority,
			IdempotencyKey:  &idem,
			Revision:        1 + len(plan.statusChain),
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("demoseed: operation task: %w", err)
		}
		count("operation_tasks", 1)

		var draft operationtask.PlatformDraft
		if plan.withDraft {
			draftIdem := fmt.Sprintf("DEMO-%s-DRAFT-1", plan.suffix)
			draft = operationtask.PlatformDraft{
				TenantID:        s.TenantID,
				OperationTaskID: task.ID,
				Platform:        operationtask.PlatformLocal,
				AdapterMode:     operationtask.AdapterModeLocalDraftOnly,
				DraftVersion:    1,
				Payload:         payload,
				PayloadHash:     payloadHash,
				Status:          plan.draftStatus,
				ChangeReason:    "DEMO- 种子数据草稿",
				IdempotencyKey:  &draftIdem,
			}
			if err := tx.Create(&draft).Error; err != nil {
				return fmt.Errorf("demoseed: platform draft: %w", err)
			}
			count("platform_drafts", 1)
		}

		var approval operationtask.ApprovalRecord
		if plan.approvalDecision != "" {
			approvalIdem := fmt.Sprintf("DEMO-%s-APPROVAL-1", plan.suffix)
			approval = operationtask.ApprovalRecord{
				TenantID:         s.TenantID,
				OperationTaskID:  task.ID,
				PlatformDraftID:  draft.ID,
				Decision:         plan.approvalDecision,
				DraftVersion:     1,
				DraftPayloadHash: payloadHash,
				ReviewerID:       reviewerID,
				ReviewerRole:     operationtask.ReviewerRoleAdmin,
				Reason:           "DEMO- 种子数据审核记录",
				RequestID:        fmt.Sprintf("DEMO-REQ-%s-APPROVAL", plan.suffix),
				IdempotencyKey:   &approvalIdem,
			}
			if err := tx.Create(&approval).Error; err != nil {
				return fmt.Errorf("demoseed: approval record: %w", err)
			}
			count("approval_records", 1)
		}

		if plan.withAttempt {
			attemptIdem := fmt.Sprintf("DEMO-%s-ATTEMPT-1", plan.suffix)
			startedAt := now.Add(-time.Duration(20+i) * time.Minute)
			finishedAt := startedAt.Add(2 * time.Minute)
			attempt := operationtask.ExecutionAttempt{
				TenantID:                 s.TenantID,
				OperationTaskID:          task.ID,
				PlatformDraftID:          draft.ID,
				ApprovalRecordID:         approval.ID,
				AttemptNumber:            1,
				Status:                   plan.attemptStatus,
				AdapterMode:              operationtask.AdapterModeLocalDraftOnly,
				Platform:                 operationtask.PlatformLocal,
				ApprovedDraftVersion:     1,
				ApprovedDraftPayloadHash: payloadHash,
				ExecutedDraftVersion:     1,
				ExecutedDraftPayloadHash: payloadHash,
				RequestID:                fmt.Sprintf("DEMO-REQ-%s-EXEC", plan.suffix),
				IdempotencyKey:           &attemptIdem,
				SafeMetadata:             mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
				Revision:                 1,
				StartedAt:                &startedAt,
				FinishedAt:               &finishedAt,
			}
			if plan.attemptStatus == operationtask.ExecutionAttemptStatusSucceeded {
				attempt.ResultType = "local_draft_written"
			}
			if err := tx.Create(&attempt).Error; err != nil {
				return fmt.Errorf("demoseed: execution attempt: %w", err)
			}
			count("execution_attempts", 1)

			if plan.attemptStatus == operationtask.ExecutionAttemptStatusFailed {
				execErr := operationtask.ExecutionError{
					TenantID:           s.TenantID,
					ExecutionAttemptID: attempt.ID,
					Sequence:           1,
					Category:           plan.errCategory,
					Code:               plan.errCode,
					SafeMessage:        plan.errMessage,
					Retryable:          plan.errRetryable,
					Details:            mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
					OccurredAt:         finishedAt,
				}
				if err := tx.Create(&execErr).Error; err != nil {
					return fmt.Errorf("demoseed: execution error: %w", err)
				}
				count("execution_errors", 1)
			}
		}

		// audit timeline: task_created plus one event per status transition
		events := []operationtask.OperationTaskEvent{{
			TenantID:        s.TenantID,
			OperationTaskID: task.ID,
			Sequence:        1,
			EventType:       operationtask.OperationTaskEventTypeTaskCreated,
			ActorType:       operationtask.OperationTaskEventActorAI,
			AfterState:      operationtask.OperationTaskStatusSuggested,
			RequestID:       fmt.Sprintf("DEMO-REQ-%s-1", plan.suffix),
			Metadata:        mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
			OccurredAt:      now.Add(-time.Duration(60-i) * time.Minute),
		}}
		from := operationtask.OperationTaskStatusSuggested
		for step, to := range plan.statusChain {
			ev := operationtask.OperationTaskEvent{
				TenantID:        s.TenantID,
				OperationTaskID: task.ID,
				Sequence:        step + 2,
				EventType:       eventTypeForTransition(to),
				ActorType:       operationtask.OperationTaskEventActorSystem,
				BeforeState:     from,
				AfterState:      to,
				RequestID:       fmt.Sprintf("DEMO-REQ-%s-%d", plan.suffix, step+2),
				Metadata:        mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
				OccurredAt:      now.Add(-time.Duration(58-i-step) * time.Minute),
			}
			if to == operationtask.OperationTaskStatusApproved || to == operationtask.OperationTaskStatusRejected {
				ev.ActorType = operationtask.OperationTaskEventActorUser
				ev.ActorID = &reviewerID
				ev.Reason = "DEMO- 种子数据审核"
			}
			if plan.withDraft {
				ev.PlatformDraftID = &draft.ID
				ev.DraftVersion = 1
			}
			events = append(events, ev)
			from = to
		}
		for i := range events {
			if err := tx.Create(&events[i]).Error; err != nil {
				return fmt.Errorf("demoseed: operation task event: %w", err)
			}
		}
		count("operation_task_events", int64(len(events)))
	}
	return nil
}

// cleanupOperationTasks hard-deletes DEMO- operation tasks with all children.
// Demo tasks are matched by DEMO- title / idempotency key; children are
// matched by task ownership OR their own DEMO- markers, so approvals or
// attempts created from the UI against demo tasks are cleaned too. Immutable
// audit guards are lifted only inside this trusted cleanup transaction.
func cleanupOperationTasks(tx *gorm.DB, res *FullDemoResult, like string) error {
	if !tx.Migrator().HasTable("operation_tasks") {
		return nil
	}
	var taskIDs []uuid.UUID
	if err := tx.Model(&operationtask.OperationTask{}).
		Where("title LIKE ? OR idempotency_key LIKE ?", like, like).
		Pluck("id", &taskIDs).Error; err != nil {
		return err
	}
	del := func(table string, q *gorm.DB) error {
		if q.Error != nil {
			return fmt.Errorf("demoseed cleanup %s: %w", table, q.Error)
		}
		res.Counts[table] += q.RowsAffected
		return nil
	}
	return operationtask.WithImmutableGuardsDisabled(tx, func(tx *gorm.DB) error {
		if len(taskIDs) > 0 {
			var attemptIDs []uuid.UUID
			if err := tx.Model(&operationtask.ExecutionAttempt{}).
				Where("operation_task_id IN ?", taskIDs).
				Pluck("id", &attemptIDs).Error; err != nil {
				return err
			}
			if len(attemptIDs) > 0 {
				// raw deletes bypass the model-level immutable hooks; the
				// database guards are lifted by WithImmutableGuardsDisabled.
				if err := del("execution_errors",
					tx.Exec(`DELETE FROM execution_errors WHERE execution_attempt_id IN ?`, attemptIDs)); err != nil {
					return err
				}
			}
			if err := del("execution_attempts",
				tx.Unscoped().Where("operation_task_id IN ?", taskIDs).Delete(&operationtask.ExecutionAttempt{})); err != nil {
				return err
			}
			if err := del("approval_records",
				tx.Exec(`DELETE FROM approval_records WHERE operation_task_id IN ?`, taskIDs)); err != nil {
				return err
			}
			if err := del("operation_task_events",
				tx.Exec(`DELETE FROM operation_task_events WHERE operation_task_id IN ?`, taskIDs)); err != nil {
				return err
			}
			if err := del("platform_drafts",
				tx.Unscoped().Where("operation_task_id IN ?", taskIDs).Delete(&operationtask.PlatformDraft{})); err != nil {
				return err
			}
			if err := del("operation_tasks",
				tx.Unscoped().Where("id IN ?", taskIDs).Delete(&operationtask.OperationTask{})); err != nil {
				return err
			}
		}
		// residual DEMO- marked children whose parent task is already gone
		if err := del("execution_errors",
			tx.Exec(`DELETE FROM execution_errors WHERE safe_message LIKE ?`, like)); err != nil {
			return err
		}
		if err := del("execution_attempts",
			tx.Unscoped().Where("idempotency_key LIKE ? OR request_id LIKE ?", like, like).Delete(&operationtask.ExecutionAttempt{})); err != nil {
			return err
		}
		if err := del("approval_records",
			tx.Exec(`DELETE FROM approval_records WHERE idempotency_key LIKE ? OR request_id LIKE ?`, like, like)); err != nil {
			return err
		}
		if err := del("operation_task_events",
			tx.Exec(`DELETE FROM operation_task_events WHERE request_id LIKE ?`, like)); err != nil {
			return err
		}
		if err := del("platform_drafts",
			tx.Unscoped().Where("idempotency_key LIKE ?", like).Delete(&operationtask.PlatformDraft{})); err != nil {
			return err
		}
		return nil
	})
}

// operationTaskVerifyChecks returns residual DEMO- row counters for the
// operation task tables (all zero after cleanup).
func operationTaskVerifyChecks(tx *gorm.DB, like string) []verifyCheck {
	if !tx.Migrator().HasTable("operation_tasks") {
		return nil
	}
	return []verifyCheck{
		{table: "operation_tasks", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.OperationTask{}).
				Where("title LIKE ? OR idempotency_key LIKE ?", like, like).Count(&n).Error
		}},
		{table: "platform_drafts", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.PlatformDraft{}).
				Where("idempotency_key LIKE ? OR operation_task_id IN (?)", like,
					tx.Model(&operationtask.OperationTask{}).Select("id").
						Where("title LIKE ? OR idempotency_key LIKE ?", like, like)).Count(&n).Error
		}},
		{table: "approval_records", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.ApprovalRecord{}).
				Where("idempotency_key LIKE ? OR request_id LIKE ? OR operation_task_id IN (?)", like, like,
					tx.Model(&operationtask.OperationTask{}).Select("id").
						Where("title LIKE ? OR idempotency_key LIKE ?", like, like)).Count(&n).Error
		}},
		{table: "execution_attempts", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.ExecutionAttempt{}).
				Where("idempotency_key LIKE ? OR request_id LIKE ? OR operation_task_id IN (?)", like, like,
					tx.Model(&operationtask.OperationTask{}).Select("id").
						Where("title LIKE ? OR idempotency_key LIKE ?", like, like)).Count(&n).Error
		}},
		{table: "execution_errors", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.ExecutionError{}).
				Where("safe_message LIKE ?", like).Count(&n).Error
		}},
		{table: "operation_task_events", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&operationtask.OperationTaskEvent{}).
				Where("request_id LIKE ? OR operation_task_id IN (?)", like,
					tx.Model(&operationtask.OperationTask{}).Select("id").
						Where("title LIKE ? OR idempotency_key LIKE ?", like, like)).Count(&n).Error
		}},
	}
}
