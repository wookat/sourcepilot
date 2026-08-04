package bannedwords

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound is returned when a word is absent in the tenant scope.
var ErrNotFound = errors.New("banned word not found")

// Service owns banned word CRUD and product scanning with tenant isolation.
type Service struct {
	DB    *gorm.DB
	OpLog *operationlog.Service
}

// EnsurePresets idempotently seeds the built-in library for one tenant.
func EnsurePresets(ctx context.Context, db *gorm.DB, tenantID int64) error {
	if db == nil {
		return fmt.Errorf("bannedwords: no db")
	}
	presets := Presets()
	rows := make([]BannedWord, 0, len(presets))
	for _, p := range presets {
		rows = append(rows, BannedWord{
			TenantID:   tenantID,
			Word:       p.Word,
			Category:   p.Category,
			Level:      p.Level,
			IsPreset:   true,
			Enabled:    true,
			Suggestion: p.Suggestion,
		})
	}
	if err := db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "word"}}, DoNothing: true}).
		Create(&rows).Error; err != nil {
		return err
	}
	states := make([]BannedWordCategoryState, 0, len(Categories()))
	for _, cat := range Categories() {
		states = append(states, BannedWordCategoryState{TenantID: tenantID, Category: cat, Enabled: true})
	}
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "category"}}, DoNothing: true}).
		Create(&states).Error
}

// ListQuery filters GET /banned-words.
type ListQuery struct {
	Category string
	Level    string
	Keyword  string
	// EnabledOnly returns only enabled words.
	EnabledOnly bool
}

// List returns tenant words ordered by level then word, seeding presets first.
func (s *Service) List(c *gin.Context, q ListQuery) ([]BannedWord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := EnsurePresets(c.Request.Context(), s.DB, tid); err != nil {
		return nil, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", tid)
	if cat := strings.TrimSpace(q.Category); cat != "" {
		tx = tx.Where("category = ?", cat)
	}
	if lv := strings.TrimSpace(q.Level); lv != "" {
		tx = tx.Where("level = ?", lv)
	}
	if q.EnabledOnly {
		tx = tx.Where("enabled = ?", true)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		tx = tx.Where("LOWER(word) LIKE ?", "%"+strings.ToLower(kw)+"%")
	}
	var rows []BannedWord
	if err := tx.Order("is_preset DESC, level ASC, word ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateBody POST /banned-words.
type CreateBody struct {
	Word       string `json:"word"`
	Category   string `json:"category"`
	Level      string `json:"level"`
	Suggestion string `json:"suggestion"`
}

func normalizeLevel(raw string) (string, error) {
	lv := strings.ToLower(strings.TrimSpace(raw))
	if lv == "" {
		return LevelForbidden, nil
	}
	if lv != LevelForbidden && lv != LevelWarning {
		return "", fmt.Errorf("级别仅支持 forbidden（禁止）或 warning（警告）")
	}
	return lv, nil
}

func normalizeCategory(raw string) string {
	cat := strings.ToLower(strings.TrimSpace(raw))
	if cat == "" {
		return "custom"
	}
	return cat
}

// Create adds a custom banned word in the current tenant.
func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*BannedWord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	word := strings.TrimSpace(body.Word)
	if word == "" {
		return nil, fmt.Errorf("违禁词不能为空")
	}
	if len([]rune(word)) > 64 {
		return nil, fmt.Errorf("违禁词长度不能超过 64 个字符")
	}
	level, err := normalizeLevel(body.Level)
	if err != nil {
		return nil, err
	}
	var exists int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&BannedWord{}).
		Where("tenant_id = ? AND word = ?", tid, word).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, fmt.Errorf("违禁词已存在：%s", word)
	}
	row := BannedWord{
		TenantID:   tid,
		Word:       word,
		Category:   normalizeCategory(body.Category),
		Level:      level,
		Enabled:    true,
		Suggestion: strings.TrimSpace(body.Suggestion),
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	s.log(c, adminID, "banned_word.create", row.ID.String(), fmt.Sprintf("word=%s level=%s", row.Word, row.Level))
	return &row, nil
}

// UpdateBody PUT /banned-words/:id (partial semantics).
type UpdateBody struct {
	Enabled    *bool   `json:"enabled"`
	Level      *string `json:"level"`
	Category   *string `json:"category"`
	Suggestion *string `json:"suggestion"`
}

// Update toggles / edits a word. Presets only allow enable/disable.
func (s *Service) Update(c *gin.Context, id uuid.UUID, body UpdateBody, adminID *uuid.UUID) (*BannedWord, error) {
	row, err := s.findScoped(c, id)
	if err != nil {
		return nil, err
	}
	if row.IsPreset && (body.Level != nil || body.Category != nil || body.Suggestion != nil) {
		return nil, fmt.Errorf("预置违禁词只读，仅支持启用/停用")
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.Level != nil {
		level, err := normalizeLevel(*body.Level)
		if err != nil {
			return nil, err
		}
		row.Level = level
	}
	if body.Category != nil {
		row.Category = normalizeCategory(*body.Category)
	}
	if body.Suggestion != nil {
		row.Suggestion = strings.TrimSpace(*body.Suggestion)
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(row).Error; err != nil {
		return nil, err
	}
	s.log(c, adminID, "banned_word.update", row.ID.String(), fmt.Sprintf("word=%s enabled=%v", row.Word, row.Enabled))
	return row, nil
}

// Delete removes a custom word; presets can only be disabled.
func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findScoped(c, id)
	if err != nil {
		return err
	}
	if row.IsPreset {
		return fmt.Errorf("预置违禁词不可删除，可停用")
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&BannedWord{}).Error; err != nil {
		return err
	}
	s.log(c, adminID, "banned_word.delete", row.ID.String(), fmt.Sprintf("word=%s", row.Word))
	return nil
}

func (s *Service) findScoped(c *gin.Context, id uuid.UUID) (*BannedWord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row BannedWord
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// CategoryInfo is one category with its tenant switch and word counts.
type CategoryInfo struct {
	Category      string `json:"category"`
	CategoryLabel string `json:"categoryLabel"`
	Enabled       bool   `json:"enabled"`
	WordCount     int64  `json:"wordCount"`
}

// ListCategories returns built-in and custom categories with tenant states.
func (s *Service) ListCategories(c *gin.Context) ([]CategoryInfo, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := EnsurePresets(c.Request.Context(), s.DB, tid); err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	var states []BannedWordCategoryState
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tid).Find(&states).Error; err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, st := range states {
		enabled[st.Category] = st.Enabled
	}
	type catCount struct {
		Category string
		N        int64
	}
	var counts []catCount
	if err := s.DB.WithContext(ctx).Model(&BannedWord{}).
		Select("category, COUNT(*) AS n").Where("tenant_id = ?", tid).
		Group("category").Scan(&counts).Error; err != nil {
		return nil, err
	}
	countMap := map[string]int64{}
	ordered := Categories()
	seen := map[string]bool{}
	for _, cat := range ordered {
		seen[cat] = true
	}
	for _, cc := range counts {
		countMap[cc.Category] = cc.N
		if !seen[cc.Category] {
			ordered = append(ordered, cc.Category)
			seen[cc.Category] = true
		}
	}
	out := make([]CategoryInfo, 0, len(ordered))
	for _, cat := range ordered {
		en, ok := enabled[cat]
		if !ok {
			en = true
		}
		out = append(out, CategoryInfo{
			Category:      cat,
			CategoryLabel: CategoryLabel(cat),
			Enabled:       en,
			WordCount:     countMap[cat],
		})
	}
	return out, nil
}

// ToggleCategory enables/disables one category for the current tenant.
func (s *Service) ToggleCategory(c *gin.Context, category string, enabled bool, adminID *uuid.UUID) (*CategoryInfo, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	cat := strings.ToLower(strings.TrimSpace(category))
	if cat == "" {
		return nil, fmt.Errorf("分类不能为空")
	}
	var wordCount int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&BannedWord{}).
		Where("tenant_id = ? AND category = ?", tid, cat).Count(&wordCount).Error; err != nil {
		return nil, err
	}
	builtin := false
	for _, known := range Categories() {
		if known == cat {
			builtin = true
			break
		}
	}
	if !builtin && wordCount == 0 {
		return nil, ErrNotFound
	}
	ctx := c.Request.Context()
	res := s.DB.WithContext(ctx).Model(&BannedWordCategoryState{}).
		Where("tenant_id = ? AND category = ?", tid, cat).
		Update("enabled", enabled)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		state := BannedWordCategoryState{TenantID: tid, Category: cat, Enabled: enabled}
		if err := s.DB.WithContext(ctx).
			Select("ID", "TenantID", "Category", "Enabled", "CreatedAt", "UpdatedAt").
			Create(&state).Error; err != nil {
			return nil, err
		}
	}
	s.log(c, adminID, "banned_word.category_toggle", cat, fmt.Sprintf("category=%s enabled=%v", cat, enabled))
	return &CategoryInfo{Category: cat, CategoryLabel: CategoryLabel(cat), Enabled: enabled, WordCount: wordCount}, nil
}

// ActiveWords returns enabled words in enabled categories for one tenant.
func (s *Service) ActiveWords(ctx context.Context, tenantID int64) ([]BannedWord, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	if err := EnsurePresets(ctx, s.DB, tenantID); err != nil {
		return nil, err
	}
	var disabled []string
	if err := s.DB.WithContext(ctx).Model(&BannedWordCategoryState{}).
		Where("tenant_id = ? AND enabled = ?", tenantID, false).
		Pluck("category", &disabled).Error; err != nil {
		return nil, err
	}
	tx := s.DB.WithContext(ctx).Where("tenant_id = ? AND enabled = ?", tenantID, true)
	if len(disabled) > 0 {
		tx = tx.Where("category NOT IN ?", disabled)
	}
	var rows []BannedWord
	if err := tx.Order("level ASC, word ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ScanResult aggregates one product's banned-word scan.
type ScanResult struct {
	ProductID      uuid.UUID `json:"productId"`
	Status         string    `json:"status"` // passed / warning / blocked
	StatusLabel    string    `json:"statusLabel"`
	ForbiddenCount int       `json:"forbiddenCount"`
	WarningCount   int       `json:"warningCount"`
	Hits           []Hit     `json:"hits"`
	// Fields echoes the scanned texts so the UI can highlight positions.
	Fields []FieldText `json:"fields"`
}

// ProductFields lists the scannable draft copy fields of a product.
func ProductFields(p product.Product) []FieldText {
	return []FieldText{
		{Field: "title", Label: "商品标题", Text: p.Title},
		{Field: "aiTitle", Label: "AI 标题", Text: p.AITitle},
		{Field: "description", Label: "商品详情/卖点", Text: p.Description},
		{Field: "aiDescription", Label: "AI 描述", Text: p.AIDescription},
	}
}

// ScanProduct scans one product's draft copy against the tenant library.
func (s *Service) ScanProduct(ctx context.Context, p product.Product) (*ScanResult, error) {
	words, err := s.ActiveWords(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	fields := ProductFields(p)
	hits := Scan(fields, words)
	res := &ScanResult{ProductID: p.ID, Hits: hits, Fields: fields}
	for _, h := range hits {
		switch h.Level {
		case LevelForbidden:
			res.ForbiddenCount++
		case LevelWarning:
			res.WarningCount++
		}
	}
	switch {
	case res.ForbiddenCount > 0:
		res.Status = "blocked"
		res.StatusLabel = "存在禁止级违禁词"
	case res.WarningCount > 0:
		res.Status = "warning"
		res.StatusLabel = "存在警告级违禁词"
	default:
		res.Status = "passed"
		res.StatusLabel = "未检出违禁词"
	}
	return res, nil
}

// FindProductScoped loads a product in the current tenant (absent → ErrNotFound).
func (s *Service) FindProductScoped(c *gin.Context, id uuid.UUID) (*product.Product, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("bannedwords: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var prod product.Product
	if err := s.DB.WithContext(c.Request.Context()).
		First(&prod, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &prod, nil
}

func (s *Service) log(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "banned_word",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
