package selection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"github.com/trademind-ai/trademind/backend/internal/providers/marketprice"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourcematch"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// Errors surfaced to the handler layer.
var (
	ErrQueueUnavailable = errors.New("selection: redis queue unavailable")
	ErrNotFound         = errors.New("selection: not found")
	ErrNotScored        = errors.New("selection: candidate not scored yet")
	ErrAlreadyDrafted   = errors.New("selection: candidate already converted to draft")
	ErrNotApproved      = errors.New("selection: candidate not approved")
)

const maxTaskCandidates = 200

// Service orchestrates the selection pipeline.
type Service struct {
	DB        *gorm.DB
	Redis     *rdb.Client
	QueueName string
	Log       *slog.Logger

	Products *product.Service
	Settings *settings.Service
	OpLog    *operationlog.Service

	AIGateway AIGatewayIface
	Prompts   PromptReader

	MarketMock  marketprice.Provider
	SourceMock  sourcematch.Provider
	SourceCrawl sourcematch.Provider
	SourceOpen  sourcematch.Provider

	TaskLeaseTimeoutSeconds int
}

func (s *Service) sourceProvider(name string) sourcematch.Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "crawler":
		return s.SourceCrawl
	case "open1688":
		return s.SourceOpen
	default:
		return s.SourceMock
	}
}

// CreateTask persists the task + candidates and enqueues the pipeline run.
func (s *Service) CreateTask(c *gin.Context, body CreateTaskBody, adminID *uuid.UUID) (*SelectionTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	ctx := c.Request.Context()
	platform := strings.TrimSpace(body.TargetPlatform)
	if platform == "" {
		return nil, fmt.Errorf("targetPlatform is required")
	}
	items := buildItems(body)
	if len(items) == 0 && len(body.ProductIDs) == 0 {
		return nil, fmt.Errorf("at least one of items/keywords/productIds is required")
	}
	if len(items)+len(body.ProductIDs) > maxTaskCandidates {
		return nil, fmt.Errorf("too many candidates (max %d)", maxTaskCandidates)
	}

	var paramsJSON datatypes.JSON
	if body.Params != nil {
		b, err := json.Marshal(body.Params)
		if err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		paramsJSON = b
	}

	country := strings.ToUpper(strings.TrimSpace(body.TargetCountry))
	if country == "" {
		country = "US"
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = fmt.Sprintf("选品任务 %s", time.Now().Format("2006-01-02 15:04"))
	}
	task := &SelectionTask{
		Name:           name,
		TargetPlatform: platform,
		TargetCountry:  country,
		Status:         StatusPending,
		Params:         paramsJSON,
		MaxRetries:     3,
		CreatedBy:      adminID,
	}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		for _, it := range items {
			cand := &SelectionCandidate{
				TaskID:         task.ID,
				Title:          strings.TrimSpace(it.Title),
				ImageURL:       strings.TrimSpace(it.ImageURL),
				Category:       strings.TrimSpace(it.Category),
				SourceURL:      strings.TrimSpace(it.SourceURL),
				MarketPlatform: platform,
				MarketPrice:    it.MarketPrice,
				MarketCurrency: strings.ToUpper(strings.TrimSpace(it.MarketCurrency)),
				MarketSales30d: it.MarketSales30d,
				Status:         CandidatePending,
			}
			if err := tx.Create(cand).Error; err != nil {
				return err
			}
		}
		for _, pid := range body.ProductIDs {
			u, err := uuid.Parse(strings.TrimSpace(pid))
			if err != nil {
				return fmt.Errorf("invalid productId %q", pid)
			}
			var p product.Product
			if err := tx.First(&p, "id = ?", u).Error; err != nil {
				return fmt.Errorf("product %s not found", pid)
			}
			img := ""
			var imgRow product.ProductImage
			if err := tx.Where("product_id = ?", p.ID).Order("created_at ASC").First(&imgRow).Error; err == nil {
				img = imgRow.PublicURL
				if img == "" {
					img = imgRow.OriginURL
				}
			}
			cand := &SelectionCandidate{
				TaskID:         task.ID,
				ProductID:      &p.ID,
				Title:          p.Title,
				ImageURL:       img,
				SourceURL:      p.SourceURL,
				MarketPlatform: platform,
				Status:         CandidatePending,
			}
			if err := tx.Create(cand).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.enqueueTask(ctx, task.ID); err != nil {
		_ = s.DB.WithContext(ctx).Where("task_id = ?", task.ID).Delete(&SelectionCandidate{}).Error
		_ = s.DB.WithContext(ctx).Where("id = ?", task.ID).Delete(&SelectionTask{}).Error
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "selection.task.create",
			Resource:   "selection_task",
			ResourceID: task.ID.String(),
			Status:     "success",
			Message:    fmt.Sprintf("platform=%s candidates=%d", platform, len(items)+len(body.ProductIDs)),
		})
	}
	return task, nil
}

func buildItems(body CreateTaskBody) []CandidateItem {
	items := make([]CandidateItem, 0, len(body.Items)+len(body.Keywords))
	for _, it := range body.Items {
		if strings.TrimSpace(it.Title) == "" {
			continue
		}
		items = append(items, it)
	}
	for _, kw := range body.Keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		items = append(items, CandidateItem{Title: kw})
	}
	return items
}

// ListTasks returns a paged task list with candidate counters.
func (s *Service) ListTasks(ctx context.Context, page, pageSize int, status string) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	tx := s.DB.WithContext(ctx).Model(&SelectionTask{})
	if v := strings.TrimSpace(status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []SelectionTask
	if err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, s.taskDTO(ctx, &rows[i]))
	}
	return &ListResult{Items: out, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetTask returns one task with counters.
func (s *Service) GetTask(ctx context.Context, id uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	var row SelectionTask
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	dto := s.taskDTO(ctx, &row)
	return &dto, nil
}

func (s *Service) taskDTO(ctx context.Context, t *SelectionTask) TaskDTO {
	var total, scored, failed int64
	base := s.DB.WithContext(ctx).Model(&SelectionCandidate{}).Where("task_id = ?", t.ID)
	_ = base.Session(&gorm.Session{}).Count(&total).Error
	_ = base.Session(&gorm.Session{}).Where("status = ?", CandidateScored).Count(&scored).Error
	_ = base.Session(&gorm.Session{}).Where("status = ?", CandidateFailed).Count(&failed).Error
	return TaskDTO{
		ID:             t.ID,
		Name:           t.Name,
		TargetPlatform: t.TargetPlatform,
		TargetCountry:  t.TargetCountry,
		Status:         t.Status,
		Params:         t.Params,
		ErrorMessage:   t.ErrorMessage,
		CandidateCount: total,
		ScoredCount:    scored,
		FailedCount:    failed,
		CreatedAt:      t.CreatedAt,
		StartedAt:      t.StartedAt,
		FinishedAt:     t.FinishedAt,
	}
}

// ListCandidates returns candidates of a task, ranked by AI score desc (可上架清单).
func (s *Service) ListCandidates(ctx context.Context, taskID uuid.UUID) ([]CandidateDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	var cands []SelectionCandidate
	if err := s.DB.WithContext(ctx).Where("task_id = ?", taskID).Find(&cands).Error; err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return []CandidateDTO{}, nil
	}
	ids := make([]uuid.UUID, 0, len(cands))
	for i := range cands {
		ids = append(ids, cands[i].ID)
	}
	var evals []SelectionEvaluation
	if err := s.DB.WithContext(ctx).Where("candidate_id IN ?", ids).Find(&evals).Error; err != nil {
		return nil, err
	}
	evalBy := map[uuid.UUID]*SelectionEvaluation{}
	for i := range evals {
		evalBy[evals[i].CandidateID] = &evals[i]
	}
	var matches []SelectionSourceMatch
	if err := s.DB.WithContext(ctx).Where("candidate_id IN ?", ids).Order("similarity DESC").Find(&matches).Error; err != nil {
		return nil, err
	}
	matchBy := map[uuid.UUID][]SelectionSourceMatch{}
	for i := range matches {
		matchBy[matches[i].CandidateID] = append(matchBy[matches[i].CandidateID], matches[i])
	}

	out := make([]CandidateDTO, 0, len(cands))
	for i := range cands {
		dto := CandidateDTO{Candidate: cands[i]}
		if ev := evalBy[cands[i].ID]; ev != nil {
			dto.Evaluation = ev
		}
		ms := matchBy[cands[i].ID]
		dto.Matches = ms
		if dto.Evaluation != nil && dto.Evaluation.BestMatchID != nil {
			for j := range ms {
				if ms[j].ID == *dto.Evaluation.BestMatchID {
					dto.BestMatch = &ms[j]
					break
				}
			}
		}
		if dto.BestMatch == nil && len(ms) > 0 {
			dto.BestMatch = &ms[0]
		}
		out = append(out, dto)
	}
	// Rank: scored first (score desc), then others by created order.
	sortCandidates(out)
	return out, nil
}

func sortCandidates(list []CandidateDTO) {
	score := func(d *CandidateDTO) float64 {
		if d.Evaluation != nil && d.Evaluation.AIScore != nil {
			return *d.Evaluation.AIScore
		}
		return -1
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && score(&list[j]) > score(&list[j-1]); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// Decide records the人工审核 decision on a candidate's evaluation.
func (s *Service) Decide(c *gin.Context, candidateID uuid.UUID, decision string, adminID *uuid.UUID) (*SelectionEvaluation, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != DecisionApproved && decision != DecisionRejected {
		return nil, fmt.Errorf("decision must be approved or rejected")
	}
	ctx := c.Request.Context()
	var ev SelectionEvaluation
	if err := s.DB.WithContext(ctx).First(&ev, "candidate_id = ?", candidateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotScored
		}
		return nil, err
	}
	now := time.Now().UTC()
	ev.Decision = decision
	ev.DecidedBy = adminID
	ev.DecidedAt = &now
	if err := s.DB.WithContext(ctx).Model(&SelectionEvaluation{}).Where("id = ?", ev.ID).
		Updates(map[string]any{"decision": decision, "decided_by": adminID, "decided_at": now}).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "selection.candidate.decision",
			Resource:   "selection_candidate",
			ResourceID: candidateID.String(),
			Status:     "success",
			Message:    decision,
		})
	}
	return &ev, nil
}

// ToDraft converts an approved candidate into a product draft (idempotent:
// returns the existing draft if already converted).
func (s *Service) ToDraft(c *gin.Context, candidateID uuid.UUID, adminID *uuid.UUID) (*product.Product, error) {
	if s == nil || s.DB == nil || s.Products == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	ctx := c.Request.Context()
	var cand SelectionCandidate
	if err := s.DB.WithContext(ctx).First(&cand, "id = ?", candidateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var ev SelectionEvaluation
	if err := s.DB.WithContext(ctx).First(&ev, "candidate_id = ?", candidateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotScored
		}
		return nil, err
	}
	if ev.DraftProductID != nil {
		var existing product.Product
		if err := s.DB.WithContext(ctx).First(&existing, "id = ?", *ev.DraftProductID).Error; err == nil {
			return &existing, ErrAlreadyDrafted
		}
	}
	if ev.Decision != DecisionApproved {
		return nil, ErrNotApproved
	}

	var best *SelectionSourceMatch
	if ev.BestMatchID != nil {
		var m SelectionSourceMatch
		if err := s.DB.WithContext(ctx).First(&m, "id = ?", *ev.BestMatchID).Error; err == nil {
			best = &m
		}
	}
	provenance := map[string]any{
		"selectionCandidateId": cand.ID.String(),
		"selectionTaskId":      cand.TaskID.String(),
		"evaluation":           ev,
	}
	if best != nil {
		provenance["bestMatch"] = best
	}
	rawJSON, err := json.Marshal(map[string]any{"selection": provenance})
	if err != nil {
		return nil, err
	}

	currency := cand.MarketCurrency
	if currency == "" {
		currency = "USD"
	}
	params := product.ImportDraftParams{
		Source:             "selection",
		SourceURL:          cand.SourceURL,
		Title:              cand.Title,
		Currency:           currency,
		FullNormalizedJSON: rawJSON,
	}
	if best != nil && best.SourceURL != "" && params.SourceURL == "" {
		params.SourceURL = best.SourceURL
	}
	if cand.ImageURL != "" {
		params.MainImages = []string{cand.ImageURL}
	}
	if cand.MarketPrice != nil {
		price := *cand.MarketPrice
		var cost *float64
		if best != nil && best.MinPrice != nil && ev.ExchangeRate != nil && *ev.ExchangeRate > 0 {
			v := round2(*best.MinPrice / *ev.ExchangeRate)
			cost = &v
		}
		params.SKUs = []product.ImportSKUParams{{
			SKUCode:   "DEFAULT",
			SKUName:   truncate(cand.Title, 60),
			Price:     &price,
			CostPrice: cost,
		}}
	}

	created, err := s.Products.ImportDraftWithContext(ctx, adminID, params)
	if err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&SelectionEvaluation{}).Where("id = ?", ev.ID).
		Update("draft_product_id", created.ID).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "selection.candidate.to_draft",
			Resource:   "selection_candidate",
			ResourceID: candidateID.String(),
			Status:     "success",
			Message:    fmt.Sprintf("draft=%s", created.ID.String()),
		})
	}
	return created, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// Retry re-enqueues a failed/partial task.
func (s *Service) Retry(ctx context.Context, id uuid.UUID) (*SelectionTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	var task SelectionTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if task.Status != StatusFailed && task.Status != StatusPartial {
		return nil, fmt.Errorf("only failed/partial tasks can be retried (status=%s)", task.Status)
	}
	if task.RetryCount >= task.MaxRetries {
		return nil, fmt.Errorf("max retries reached (%d)", task.MaxRetries)
	}
	updates := map[string]any{
		"status":        StatusPending,
		"retry_count":   gorm.Expr("retry_count + 1"),
		"error_message": "",
		"finished_at":   nil,
		"locked_by":     nil,
		"locked_until":  nil,
	}
	if err := s.DB.WithContext(ctx).Model(&SelectionTask{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.enqueueTask(ctx, id); err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// RunSelectionJob executes the full pipeline for one claimed task.
func (s *Service) RunSelectionJob(parent context.Context, taskID uuid.UUID, workerID string) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() {
		if r := recover(); r != nil {
			_ = s.DB.WithContext(context.Background()).Model(&SelectionTask{}).Where("id = ?", taskID).
				Updates(map[string]any{"status": StatusFailed, "error_message": fmt.Sprintf("panic: %v", r), "finished_at": time.Now().UTC()}).Error
		}
	}()

	lease := time.Duration(s.TaskLeaseTimeoutSeconds) * time.Second
	if lease <= 0 {
		lease = 300 * time.Second
	}
	claim, ok, err := tasklease.TryClaim(ctx, s.DB, SelectionTask{}.TableName(), StatusPending, StatusRunning, taskID, workerID, lease)
	if err != nil || !ok {
		return
	}
	stopRenew := tasklease.StartRenewal(ctx, s.DB, SelectionTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, lease)
	defer stopRenew()

	var task SelectionTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}
	cfg := s.resolveConfig(ctx, &task)

	var cands []SelectionCandidate
	if err := s.DB.WithContext(ctx).Where("task_id = ? AND status <> ?", taskID, CandidateScored).Find(&cands).Error; err != nil {
		s.finishTask(ctx, taskID, StatusFailed, err.Error())
		return
	}

	failed := 0
	for i := range cands {
		if ctx.Err() != nil {
			return
		}
		if err := s.processCandidate(ctx, &task, &cands[i], cfg); err != nil {
			failed++
			_ = s.DB.WithContext(ctx).Model(&SelectionCandidate{}).Where("id = ?", cands[i].ID).
				Updates(map[string]any{"status": CandidateFailed, "error_message": err.Error()}).Error
			if s.Log != nil {
				s.Log.Warn("selection_candidate_failed", "candidateId", cands[i].ID.String(), "error", err)
			}
		}
	}

	status := StatusSuccess
	msg := ""
	switch {
	case failed == len(cands) && len(cands) > 0:
		status = StatusFailed
		msg = fmt.Sprintf("all %d candidates failed", failed)
	case failed > 0:
		status = StatusPartial
		msg = fmt.Sprintf("%d/%d candidates failed", failed, len(cands))
	}
	s.finishTask(ctx, taskID, status, msg)
}

func (s *Service) finishTask(ctx context.Context, taskID uuid.UUID, status, msg string) {
	now := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&SelectionTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":        status,
		"error_message": msg,
		"finished_at":   now,
		"locked_by":     nil,
		"locked_until":  nil,
	}).Error
}

func (s *Service) resolveConfig(ctx context.Context, task *SelectionTask) EngineConfig {
	var sel, pricing map[string]string
	if s.Settings != nil {
		sel, _ = s.Settings.PlainByGroup(ctx, task.TenantID, settingsGroup)
		pricing, _ = s.Settings.PlainByGroup(ctx, task.TenantID, "pricing")
	}
	cfg := ConfigFromSettings(sel, pricing, task.TargetPlatform)
	if len(task.Params) > 0 {
		var o TaskParamOverrides
		if err := json.Unmarshal(task.Params, &o); err == nil {
			cfg.ApplyOverrides(&o)
		}
	}
	return cfg
}

// processCandidate runs市场价 → 1688匹配 → 利润 → LLM评分 for one candidate.
func (s *Service) processCandidate(ctx context.Context, task *SelectionTask, cand *SelectionCandidate, cfg EngineConfig) error {
	// 1. Overseas market price: manual value wins, otherwise provider.
	if cand.MarketPrice == nil || *cand.MarketPrice <= 0 {
		if s.MarketMock == nil {
			return fmt.Errorf("no market price and no provider")
		}
		listing, err := s.MarketMock.FetchListing(ctx, marketprice.Query{
			Platform: task.TargetPlatform,
			Country:  task.TargetCountry,
			Keyword:  cand.Title,
			ImageURL: cand.ImageURL,
		})
		if err != nil {
			return fmt.Errorf("market price: %w", err)
		}
		cand.MarketPrice = &listing.Price
		cand.MarketCurrency = listing.Currency
		cand.MarketSales30d = &listing.Sales30d
		cand.MarketRaw = datatypes.JSON(listing.Raw)
	}
	if cand.MarketCurrency == "" {
		cand.MarketCurrency = cfg.TargetCurrency
	}
	if err := s.DB.WithContext(ctx).Model(&SelectionCandidate{}).Where("id = ?", cand.ID).Updates(map[string]any{
		"market_price":    cand.MarketPrice,
		"market_currency": cand.MarketCurrency,
		"market_sales30d": cand.MarketSales30d,
		"market_raw":      cand.MarketRaw,
		"status":          CandidatePriced,
		"error_message":   "",
	}).Error; err != nil {
		return err
	}

	// 2. 1688 source match: configured provider, graceful fallback to mock.
	req := sourcematch.MatchRequest{
		Keyword:   cand.Title,
		ImageURL:  cand.ImageURL,
		Category:  cand.Category,
		SourceURL: cand.SourceURL,
		Limit:     3,
	}
	matches := s.matchSources(ctx, cfg.SourceMatchProvider, req)
	if len(matches) == 0 {
		return fmt.Errorf("no 1688 match found")
	}
	if err := s.DB.WithContext(ctx).Where("candidate_id = ?", cand.ID).Delete(&SelectionSourceMatch{}).Error; err != nil {
		return err
	}
	rows := make([]SelectionSourceMatch, 0, len(matches))
	for _, m := range matches {
		sim, minP, maxP, moq, rating := m.Similarity, m.MinPrice, m.MaxPrice, m.MOQ, m.SupplierRating
		rows = append(rows, SelectionSourceMatch{
			TenantID:       task.TenantID,
			CandidateID:    cand.ID,
			SourcePlatform: m.SourcePlatform,
			SourceURL:      m.SourceURL,
			SourceOfferID:  m.SourceOfferID,
			MatchMethod:    m.MatchMethod,
			Similarity:     &sim,
			MinPrice:       &minP,
			MaxPrice:       &maxP,
			Currency:       m.Currency,
			MOQ:            &moq,
			SupplierName:   m.SupplierName,
			SupplierRating: &rating,
			RawData:        datatypes.JSON(m.Raw),
		})
	}
	if err := s.DB.WithContext(ctx).Create(&rows).Error; err != nil {
		return err
	}
	best := &rows[0]
	for i := range rows {
		if rows[i].Similarity != nil && best.Similarity != nil && *rows[i].Similarity > *best.Similarity {
			best = &rows[i]
		}
	}
	if err := s.DB.WithContext(ctx).Model(&SelectionCandidate{}).Where("id = ?", cand.ID).
		Update("status", CandidateMatched).Error; err != nil {
		return err
	}

	// 3. Profit model.
	purchase := 0.0
	if best.MinPrice != nil {
		purchase = *best.MinPrice
	}
	profit, err := ComputeProfit(ProfitInput{
		PurchaseCostCNY: purchase,
		SellPrice:       *cand.MarketPrice,
		Params:          cfg.Profit,
	})
	if err != nil {
		return fmt.Errorf("profit: %w", err)
	}

	// 4. LLM scoring (rule fallback inside).
	in := ScoreInput{
		Title:          cand.Title,
		Category:       cand.Category,
		MarketPlatform: task.TargetPlatform,
		MarketPrice:    *cand.MarketPrice,
		MarketCurrency: cand.MarketCurrency,
		Profit:         profit,
	}
	if cand.MarketSales30d != nil {
		in.MarketSales30d = *cand.MarketSales30d
	}
	if best.MOQ != nil {
		in.MOQ = *best.MOQ
	}
	if best.SupplierRating != nil {
		in.SupplierRating = *best.SupplierRating
	}
	if best.Similarity != nil {
		in.Similarity = *best.Similarity
	}
	score := scoreCandidate(ctx, s.AIGateway, s.Prompts, in, task.TargetPlatform, task.TargetCountry)
	reasons, err := json.Marshal(score)
	if err != nil {
		return err
	}

	// 5. Upsert evaluation.
	ev := SelectionEvaluation{
		TenantID:         task.TenantID,
		CandidateID:      cand.ID,
		BestMatchID:      &best.ID,
		PurchaseCost:     &profit.PurchaseCost,
		ShippingCost:     &profit.ShippingCost,
		CommissionFee:    &profit.CommissionFee,
		ExchangeRate:     &profit.ExchangeRate,
		LandedCost:       &profit.LandedCost,
		EstProfit:        &profit.EstProfit,
		EstMarginPercent: &profit.EstMarginPercent,
		AIScore:          &score.Score,
		AIReasons:        reasons,
		AIModel:          score.Model,
		Decision:         DecisionPending,
	}
	var existing SelectionEvaluation
	err = s.DB.WithContext(ctx).First(&existing, "candidate_id = ?", cand.ID).Error
	switch {
	case err == nil:
		ev.ID = existing.ID
		ev.Decision = existing.Decision
		ev.DecidedBy = existing.DecidedBy
		ev.DecidedAt = existing.DecidedAt
		ev.DraftProductID = existing.DraftProductID
		if err := s.DB.WithContext(ctx).Model(&SelectionEvaluation{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"best_match_id":      ev.BestMatchID,
			"purchase_cost":      ev.PurchaseCost,
			"shipping_cost":      ev.ShippingCost,
			"commission_fee":     ev.CommissionFee,
			"exchange_rate":      ev.ExchangeRate,
			"landed_cost":        ev.LandedCost,
			"est_profit":         ev.EstProfit,
			"est_margin_percent": ev.EstMarginPercent,
			"ai_score":           ev.AIScore,
			"ai_reasons":         ev.AIReasons,
			"ai_model":           ev.AIModel,
		}).Error; err != nil {
			return err
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := s.DB.WithContext(ctx).Create(&ev).Error; err != nil {
			return err
		}
	default:
		return err
	}

	return s.DB.WithContext(ctx).Model(&SelectionCandidate{}).Where("id = ?", cand.ID).
		Update("status", CandidateScored).Error
}

// matchSources tries the configured provider and gracefully falls back to
// mock when it is unavailable (e.g. crawler without 1688 login profile).
func (s *Service) matchSources(ctx context.Context, providerName string, req sourcematch.MatchRequest) []sourcematch.Match {
	try := func(p sourcematch.Provider) []sourcematch.Match {
		if p == nil {
			return nil
		}
		if strings.TrimSpace(req.ImageURL) != "" {
			if ms, err := p.MatchByImage(ctx, req); err == nil && len(ms) > 0 {
				return ms
			} else if err != nil && !errors.Is(err, sourcematch.ErrUnavailable) && s.Log != nil {
				s.Log.Warn("sourcematch_image_failed", "provider", p.Name(), "error", err)
			}
		}
		if ms, err := p.MatchByKeyword(ctx, req); err == nil && len(ms) > 0 {
			return ms
		} else if err != nil && !errors.Is(err, sourcematch.ErrUnavailable) && s.Log != nil {
			s.Log.Warn("sourcematch_keyword_failed", "provider", p.Name(), "error", err)
		}
		return nil
	}
	primary := s.sourceProvider(providerName)
	if ms := try(primary); len(ms) > 0 {
		return ms
	}
	if primary != s.SourceMock {
		return try(s.SourceMock)
	}
	return nil
}
