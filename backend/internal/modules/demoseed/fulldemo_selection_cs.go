package demoseed

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

// selectionTaskPlan describes one demo selection task; statuses use the
// selection module vocabulary (pending/running/success/failed/partial).
type selectionTaskPlan struct {
	suffix         string
	name           string
	targetPlatform string
	targetCountry  string
	status         string
	errorMessage   string
	retryCount     int
	withCandidates bool // success/partial tasks carry candidate samples
}

func demoSelectionTaskPlans() []selectionTaskPlan {
	return []selectionTaskPlan{
		{suffix: "SEL-PENDING", name: "DEMO-选品任务-待执行", targetPlatform: "shopee", targetCountry: "SG", status: selection.StatusPending},
		{suffix: "SEL-RUNNING", name: "DEMO-选品任务-执行中", targetPlatform: "shopee", targetCountry: "SG", status: selection.StatusRunning},
		{suffix: "SEL-SUCCESS", name: "DEMO-选品任务-已完成", targetPlatform: "shopee", targetCountry: "SG", status: selection.StatusSuccess, withCandidates: true},
		{suffix: "SEL-PARTIAL", name: "DEMO-选品任务-部分成功", targetPlatform: "tiktok", targetCountry: "US", status: selection.StatusPartial, errorMessage: "DEMO- 演示：部分候选打分失败", withCandidates: true},
		{suffix: "SEL-FAILED", name: "DEMO-选品任务-失败", targetPlatform: "tiktok", targetCountry: "US", status: selection.StatusFailed, errorMessage: "DEMO- 演示：海外市场价拉取失败", retryCount: 3},
	}
}

// selectionCandidatePlan describes one candidate inside the completed demo
// task, including its evaluation decision (approved / rejected / pending and
// an approved-and-converted-to-draft sample).
type selectionCandidatePlan struct {
	title      string
	status     string
	decision   string
	aiScore    float64
	withDraft  bool // approved candidate already converted to a product draft
	failed     bool // candidate failed before scoring (partial task sample)
	sales30d   int
	priceLocal float64
}

func demoSelectionCandidatePlans() []selectionCandidatePlan {
	return []selectionCandidatePlan{
		{title: "DEMO-候选 北欧陶瓷马克杯 350ml", status: selection.CandidateScored,
			decision: selection.DecisionApproved, aiScore: 86.5, withDraft: true, sales30d: 1240, priceLocal: 12.9},
		{title: "DEMO-候选 便携折叠收纳箱 55L", status: selection.CandidateScored,
			decision: selection.DecisionRejected, aiScore: 41.0, sales30d: 96, priceLocal: 18.5},
		{title: "DEMO-候选 桌面LED化妆镜 带补光", status: selection.CandidateScored,
			decision: selection.DecisionPending, aiScore: 72.0, sales30d: 480, priceLocal: 15.9},
	}
}

// seedSelection inserts DEMO- selection tasks covering every status plus
// candidate / source match / evaluation samples for the completed and partial
// tasks. All rows are stamped with the seeder tenant.
func (s *FullDemoSeeder) seedSelection(tx *gorm.DB, res *FullDemoResult, now time.Time, products []product.Product) error {
	count := func(table string, n int64) { res.Counts[table] += n }
	started := now.Add(-90 * time.Minute)
	finished := now.Add(-80 * time.Minute)

	for _, plan := range demoSelectionTaskPlans() {
		task := selection.SelectionTask{TenantID: s.TenantID, Name: plan.name,
			TargetPlatform: plan.targetPlatform, TargetCountry: plan.targetCountry,
			Status: plan.status, ErrorMessage: plan.errorMessage, RetryCount: plan.retryCount,
			Params: mustJSON(map[string]any{"seedPrefix": DemoPrefix, "candidateLimit": 3})}
		if plan.status != selection.StatusPending {
			task.StartedAt = &started
		}
		if plan.status == selection.StatusSuccess || plan.status == selection.StatusPartial || plan.status == selection.StatusFailed {
			task.FinishedAt = &finished
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("demoseed: selection task: %w", err)
		}
		count("selection_tasks", 1)
		if !plan.withCandidates {
			continue
		}

		if plan.status == selection.StatusPartial {
			// one failed candidate to show the partial-failure path
			cand := selection.SelectionCandidate{TenantID: s.TenantID, TaskID: task.ID,
				Title: "DEMO-候选 打分失败样本 硅胶餐垫", MarketPlatform: plan.targetPlatform,
				Status: selection.CandidateFailed, ErrorMessage: "DEMO- 演示：AI 打分超时"}
			if err := tx.Create(&cand).Error; err != nil {
				return fmt.Errorf("demoseed: selection candidate: %w", err)
			}
			count("selection_candidates", 1)
			continue
		}

		for i, cp := range demoSelectionCandidatePlans() {
			price := cp.priceLocal
			sales := cp.sales30d
			cand := selection.SelectionCandidate{TenantID: s.TenantID, TaskID: task.ID,
				Title: cp.title, Category: "家居百货", MarketPlatform: plan.targetPlatform,
				MarketPrice: &price, MarketCurrency: "SGD", MarketSales30d: &sales,
				SourceURL: fmt.Sprintf("https://market.demo.trademind.local/item/DEMO-SEL-%d", i+1),
				Status:    cp.status}
			if err := tx.Create(&cand).Error; err != nil {
				return fmt.Errorf("demoseed: selection candidate: %w", err)
			}
			count("selection_candidates", 1)

			minP := price * 7.2 * 0.25
			maxP := minP * 1.2
			sim := 0.92 - float64(i)*0.08
			rating := 4.7
			moq := 10
			match := selection.SelectionSourceMatch{TenantID: s.TenantID, CandidateID: cand.ID,
				SourcePlatform: "1688",
				SourceURL:      fmt.Sprintf("https://detail.1688.com/offer/DEMO-SEL-%d.html", i+1),
				SourceOfferID:  fmt.Sprintf("DEMO-SEL-OFFER-%d", i+1),
				MatchMethod:    "title", Similarity: &sim,
				MinPrice: &minP, MaxPrice: &maxP, Currency: "CNY", MOQ: &moq,
				SupplierName: "DEMO-义乌市优选家居用品厂", SupplierRating: &rating}
			if err := tx.Create(&match).Error; err != nil {
				return fmt.Errorf("demoseed: selection source match: %w", err)
			}
			count("selection_source_matches", 1)

			purchase := minP
			shipping := 6.0
			commission := price * 0.08
			rate := 5.2
			landed := purchase/rate + shipping/rate + commission
			profit := price - landed
			margin := profit / price * 100
			score := cp.aiScore
			eval := selection.SelectionEvaluation{TenantID: s.TenantID, CandidateID: cand.ID,
				BestMatchID: &match.ID, PurchaseCost: &purchase, ShippingCost: &shipping,
				CommissionFee: &commission, ExchangeRate: &rate, LandedCost: &landed,
				EstProfit: &profit, EstMarginPercent: &margin,
				AIScore: &score, AIModel: "demo-mock",
				AIReasons: mustJSON([]string{"DEMO- 演示：市场需求与利润率综合评分"}),
				Decision:  cp.decision}
			if cp.decision != selection.DecisionPending {
				decidedAt := finished.Add(10 * time.Minute)
				eval.DecidedAt = &decidedAt
			}
			if cp.withDraft && len(products) > 0 {
				eval.DraftProductID = &products[0].ID
			}
			if err := tx.Create(&eval).Error; err != nil {
				return fmt.Errorf("demoseed: selection evaluation: %w", err)
			}
			count("selection_evaluations", 1)
		}
	}
	return nil
}

// customerConversationPlan describes one demo customer-service conversation
// with its message timeline and AI suggestion sample.
type customerConversationPlan struct {
	customerName string
	status       string
	language     string
	manualShop   bool // attach to the operator/readonly-granted manual DEMO shop
	messages     []struct{ role, source, content string }
	suggestion   *struct {
		status         string
		suggestedReply string
		editedReply    string
		rejectReason   string
	}
}

func demoCustomerConversationPlans() []customerConversationPlan {
	return []customerConversationPlan{
		{customerName: "DEMO-买家 Alice", status: customerchat.StatusPendingReply, language: "en",
			messages: []struct{ role, source, content string }{
				{customerchat.RoleCustomer, customerchat.SourceImported, "DEMO- Hi, when will my mug order ship?"},
			},
			suggestion: &struct {
				status         string
				suggestedReply string
				editedReply    string
				rejectReason   string
			}{status: customerchat.SuggestionGenerated,
				suggestedReply: "DEMO- Hi Alice! Your order has been packed and will ship within 24 hours."}},
		{customerName: "DEMO-买家 Bob", status: customerchat.StatusReplied, language: "en", manualShop: true,
			messages: []struct{ role, source, content string }{
				{customerchat.RoleCustomer, customerchat.SourceImported, "DEMO- Is the thermos dishwasher safe?"},
				{customerchat.RoleAgent, customerchat.SourceManual, "DEMO- Yes, the lid is dishwasher safe; hand-wash the body is recommended."},
			},
			suggestion: &struct {
				status         string
				suggestedReply string
				editedReply    string
				rejectReason   string
			}{status: customerchat.SuggestionAccepted,
				suggestedReply: "DEMO- Yes, it is dishwasher safe.",
				editedReply:    "DEMO- Yes, the lid is dishwasher safe; hand-wash the body is recommended."}},
		{customerName: "DEMO-买家 Carol", status: customerchat.StatusClosed, language: "zh", manualShop: true,
			messages: []struct{ role, source, content string }{
				{customerchat.RoleCustomer, customerchat.SourceImported, "DEMO- 请问收纳盒有黑色吗？"},
				{customerchat.RoleAgent, customerchat.SourceManual, "DEMO- 有的，黑色现货，下单当天发货。"},
				{customerchat.RoleCustomer, customerchat.SourceImported, "DEMO- 好的谢谢，已下单。"},
			},
			suggestion: &struct {
				status         string
				suggestedReply string
				editedReply    string
				rejectReason   string
			}{status: customerchat.SuggestionRejected,
				suggestedReply: "DEMO- 亲，收纳盒目前只有白色哦。",
				rejectReason:   "DEMO- 演示：AI 库存信息过时，人工改写"}},
	}
}

// demoReplyTemplatePlans returns DEMO- canned reply templates covering every
// group with variable placeholders that the workbench fills from context.
func demoReplyTemplatePlans() []customerchat.CustomerReplyTemplate {
	return []customerchat.CustomerReplyTemplate{
		{GroupKey: customerchat.TemplateGroupPresale, Name: "DEMO-售前-库存确认", SortOrder: 1, Enabled: true,
			Content: "DEMO- 亲爱的{买家昵称}您好！您咨询的{商品名}现在有现货，下单后 24 小时内发出哦～"},
		{GroupKey: customerchat.TemplateGroupPresale, Name: "DEMO-售前-尺寸咨询", SortOrder: 2, Enabled: true,
			Content: "DEMO- 您好{买家昵称}！{商品名}的详细尺寸请见商品详情页规格表，如仍有疑问请告诉我具体使用场景，我帮您推荐。"},
		{GroupKey: customerchat.TemplateGroupAftersale, Name: "DEMO-售后-使用问题", SortOrder: 1, Enabled: true,
			Content: "DEMO- {买家昵称}您好，收到您反馈的问题，请提供订单号 {订单号} 对应商品的照片或视频，我们会在 24 小时内给您处理方案。"},
		{GroupKey: customerchat.TemplateGroupLogistics, Name: "DEMO-物流-查询进度", SortOrder: 1, Enabled: true,
			Content: "DEMO- 您好{买家昵称}，您的订单 {订单号} 已发货，物流单号 {物流单号}，可在物流官网查询最新进度。"},
		{GroupKey: customerchat.TemplateGroupRefund, Name: "DEMO-退款-流程说明", SortOrder: 1, Enabled: true,
			Content: "DEMO- {买家昵称}您好，订单 {订单号} 的退款申请已收到，审核通过后 1-3 个工作日原路退回，请注意查收。"},
		{GroupKey: customerchat.TemplateGroupOther, Name: "DEMO-通用-欢迎语（停用样例）", SortOrder: 1, Enabled: false,
			Content: "DEMO- 您好{买家昵称}，欢迎光临{店铺名}，很高兴为您服务！"},
	}
}

// seedCustomerService inserts DEMO- customer conversations (message timeline +
// AI reply suggestions) and customer message sync tasks (success + failed).
// Conversations are stamped with the seeder tenant (never tenant 0).
func (s *FullDemoSeeder) seedCustomerService(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop) error {
	count := func(table string, n int64) { res.Counts[table] += n }
	demoShop := shops[0]
	manualShop := demoShop
	if len(shops) > 1 {
		manualShop = shops[1]
	}

	for ci, plan := range demoCustomerConversationPlans() {
		convShop := demoShop
		if plan.manualShop {
			convShop = manualShop
		}
		base := now.Add(-time.Duration(6-ci) * time.Hour)
		last := base.Add(time.Duration(len(plan.messages)) * 5 * time.Minute)
		conv := customerchat.CustomerConversation{TenantID: s.TenantID,
			Platform: convShop.Platform, ShopID: &convShop.ID,
			CustomerName: plan.customerName, CustomerLanguage: plan.language,
			Status: plan.status, LastMessageAt: &last}
		if err := tx.Create(&conv).Error; err != nil {
			return fmt.Errorf("demoseed: customer conversation: %w", err)
		}
		count("customer_conversations", 1)

		var lastMsgID uuid.UUID
		for mi, m := range plan.messages {
			msg := customerchat.CustomerMessage{ConversationID: conv.ID,
				Role: m.role, Content: m.content, Language: plan.language,
				MessageType: customerchat.MessageTypeText, Source: m.source,
				CreatedAt: base.Add(time.Duration(mi) * 5 * time.Minute)}
			if err := tx.Create(&msg).Error; err != nil {
				return fmt.Errorf("demoseed: customer message: %w", err)
			}
			count("customer_messages", 1)
			if m.role == customerchat.RoleCustomer {
				lastMsgID = msg.ID
			}
		}

		if plan.suggestion != nil {
			msgID := lastMsgID
			sugg := customerchat.CustomerReplySuggestion{ConversationID: conv.ID,
				MessageID: &msgID, Provider: "demo-mock", Model: "demo-mock",
				PromptCode:     "DEMO-customer_reply",
				SuggestedReply: plan.suggestion.suggestedReply,
				EditedReply:    plan.suggestion.editedReply,
				RejectReason:   plan.suggestion.rejectReason,
				Status:         plan.suggestion.status, Language: plan.language, Tone: "friendly",
				Input:  mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
				Output: mustJSON(map[string]any{"reply": plan.suggestion.suggestedReply})}
			if err := tx.Create(&sugg).Error; err != nil {
				return fmt.Errorf("demoseed: reply suggestion: %w", err)
			}
			count("customer_reply_suggestions", 1)
		}
	}

	// ---- customer reply templates（话术模板样本，覆盖全部分组 + 停用样例）----
	for _, tpl := range demoReplyTemplatePlans() {
		row := tpl
		row.TenantID = s.TenantID
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("demoseed: reply template: %w", err)
		}
		count("customer_reply_templates", 1)
	}

	// ---- customer message sync tasks（成功 + 失败样本）----
	started := now.Add(-40 * time.Minute)
	finished := now.Add(-35 * time.Minute)
	syncPlans := []struct {
		cursor, status, errMsg string
		total, ok, failed      int
	}{
		{cursor: "DEMO-CS-CURSOR-1", status: customersync.StatusSuccess, total: 12, ok: 12},
		{cursor: "DEMO-CS-CURSOR-2", status: customersync.StatusFailed,
			errMsg: "DEMO- 演示：平台消息接口限流，同步失败", total: 12, failed: 12},
	}
	for _, sp := range syncPlans {
		t := customersync.CustomerMessageSyncTask{TenantID: s.TenantID,
			ShopID: demoShop.ID, Platform: demoShop.Platform,
			TaskType: customersync.TaskTypeCustomerMessageSync,
			Status:   sp.status, Mode: customersync.ModeManual, Cursor: sp.cursor,
			StartedAt: &started, FinishedAt: &finished,
			TotalCount: sp.total, SuccessCount: sp.ok, FailedCount: sp.failed,
			ErrorMessage: sp.errMsg,
			Input:        mustJSON(map[string]any{"seedPrefix": DemoPrefix})}
		if err := tx.Create(&t).Error; err != nil {
			return fmt.Errorf("demoseed: customer sync task: %w", err)
		}
		count("customer_message_sync_tasks", 1)
	}
	return nil
}

// collectDemoSelectionCandidateIDs returns candidates belonging to DEMO- tasks
// or carrying DEMO- titles so renamed tasks still clean their children.
func collectDemoSelectionCandidateIDs(tx *gorm.DB, like string, taskIDs []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	if len(taskIDs) > 0 {
		if err := tx.Model(&selection.SelectionCandidate{}).Unscoped().
			Where("task_id IN ?", taskIDs).Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	ids = ids[:0]
	if err := tx.Model(&selection.SelectionCandidate{}).Unscoped().
		Where("title LIKE ?", like).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}

// cleanupSelection removes DEMO- selection tasks with candidates, source
// matches and evaluations.
func cleanupSelection(tx *gorm.DB, res *FullDemoResult, like string) error {
	del := func(table string, q *gorm.DB) error {
		if q.Error != nil {
			return fmt.Errorf("demoseed cleanup %s: %w", table, q.Error)
		}
		res.Counts[table] += q.RowsAffected
		return nil
	}
	var taskIDs []uuid.UUID
	if err := tx.Model(&selection.SelectionTask{}).Unscoped().
		Where("name LIKE ?", like).Pluck("id", &taskIDs).Error; err != nil {
		return err
	}
	candIDs, err := collectDemoSelectionCandidateIDs(tx, like, taskIDs)
	if err != nil {
		return err
	}
	if len(candIDs) > 0 {
		if err := del("selection_evaluations", tx.Unscoped().Where("candidate_id IN ?", candIDs).Delete(&selection.SelectionEvaluation{})); err != nil {
			return err
		}
		if err := del("selection_source_matches", tx.Unscoped().Where("candidate_id IN ?", candIDs).Delete(&selection.SelectionSourceMatch{})); err != nil {
			return err
		}
		if err := del("selection_candidates", tx.Unscoped().Where("id IN ?", candIDs).Delete(&selection.SelectionCandidate{})); err != nil {
			return err
		}
	}
	if len(taskIDs) > 0 {
		if err := del("selection_tasks", tx.Unscoped().Where("id IN ?", taskIDs).Delete(&selection.SelectionTask{})); err != nil {
			return err
		}
	}
	return nil
}

// cleanupCustomerSyncTasks removes DEMO- customer message sync tasks (cursor
// or error message carries the DEMO- marker).
func cleanupCustomerSyncTasks(tx *gorm.DB, res *FullDemoResult, like string) error {
	q := tx.Unscoped().Where("cursor LIKE ? OR error_message LIKE ?", like, like).
		Delete(&customersync.CustomerMessageSyncTask{})
	if q.Error != nil {
		return fmt.Errorf("demoseed cleanup customer_message_sync_tasks: %w", q.Error)
	}
	res.Counts["customer_message_sync_tasks"] += q.RowsAffected
	return nil
}
