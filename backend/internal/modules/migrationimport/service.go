package migrationimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Service implements the migration import flows on top of the product,
// order and sourcing module services (scope checks and persistence reuse
// their logic).
type Service struct {
	DB       *gorm.DB
	Products *product.Service
	Orders   *order.Service
	Sourcing *sourcing.Service
	Finance  *finance.Service
	OpLog    *operationlog.Service
}

// ParseResult is the response of POST /imports/parse.
type ParseResult struct {
	Kind         string         `json:"kind"`
	FileName     string         `json:"fileName"`
	FileHash     string         `json:"fileHash"`
	SourceFormat string         `json:"sourceFormat"`
	Columns      []string       `json:"columns"`
	Rows         [][]string     `json:"rows"`
	TotalRows    int            `json:"totalRows"`
	Mapping      map[string]int `json:"mapping"`
	Fields       []FieldDef     `json:"fields"`
}

// WizardBody is the shared payload of validate / commit.
type WizardBody struct {
	Kind         string         `json:"kind"`
	ShopID       string         `json:"shopId"`
	Columns      []string       `json:"columns"`
	Rows         [][]string     `json:"rows"`
	Mapping      map[string]int `json:"mapping"`
	FileName     string         `json:"fileName"`
	FileHash     string         `json:"fileHash"`
	SourceFormat string         `json:"sourceFormat"`
}

// ValidateResult is the response of POST /imports/validate.
type ValidateResult struct {
	TotalRows  int        `json:"totalRows"`
	ValidRows  int        `json:"validRows"`
	ErrorRows  int        `json:"errorRows"`
	GroupCount int        `json:"groupCount"`
	Errors     []RowError `json:"errors"`
}

// CommitResult is the response of POST /imports/commit.
type CommitResult struct {
	JobID         uuid.UUID `json:"jobId"`
	Status        string    `json:"status"`
	TotalRows     int       `json:"totalRows"`
	SuccessRows   int       `json:"successRows"`
	FailedRows    int       `json:"failedRows"`
	DuplicateRows int       `json:"duplicateRows"`
	Replayed      bool      `json:"replayed"`
}

// JobDTO is one import history row.
type JobDTO struct {
	ImportJob
	ErrorRowCount int `json:"errorRowCount"`
}

func normalizeKind(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case KindProduct, KindOrder, KindInventory, KindSource, KindPayment:
		return strings.TrimSpace(kind), nil
	default:
		return "", fmt.Errorf("kind 需为 product、order、inventory、source 或 payment")
	}
}

// kindNeedsShop reports whether an import kind is shop-scoped. Inventory and
// source-archive imports are tenant-level (SKU / supplier data has no shop).
func kindNeedsShop(kind string) bool {
	return kind == KindProduct || kind == KindOrder
}

func normalizeSourceFormat(v string) string {
	switch strings.TrimSpace(v) {
	case SourceDianxiaomi, SourceMabang:
		return strings.TrimSpace(v)
	default:
		return SourceCustom
	}
}

func (b *WizardBody) validateShape() error {
	if len(b.Columns) == 0 {
		return fmt.Errorf("columns is required")
	}
	if len(b.Rows) == 0 {
		return fmt.Errorf("rows is required")
	}
	if len(b.Rows) > MaxImportRows {
		return fmt.Errorf("单批最多导入 %d 行数据", MaxImportRows)
	}
	if b.Mapping == nil {
		return fmt.Errorf("mapping is required")
	}
	for _, f := range FieldsForKind(b.Kind) {
		idx, ok := b.Mapping[f.Key]
		if f.Required && (!ok || idx < 0) {
			return fmt.Errorf("必填字段「%s」未映射到任何列", f.Label)
		}
		if ok && idx >= len(b.Columns) {
			return fmt.Errorf("字段「%s」映射的列不存在", f.Label)
		}
	}
	return nil
}

// resolveShop parses and authorizes the target shop (required for imports;
// operator must have operate permission on the shop).
func (s *Service) resolveShop(c *gin.Context, raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("shopId is required（导入必须选择归属店铺）")
	}
	u, err := uuid.Parse(raw)
	if err != nil || u == uuid.Nil {
		return nil, fmt.Errorf("invalid shopId")
	}
	principal, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return nil, err
	}
	if !principal.IsAdmin() {
		if !principal.CanViewStore(u) {
			return nil, gorm.ErrRecordNotFound
		}
		if !principal.CanOperateStore(u) {
			return nil, errShopNotOperable
		}
	}
	return &u, nil
}

var errShopNotOperable = errors.New("当前账号无该店铺的操作权限")

var errCommitInFlight = errors.New("该批次正在导入中，请勿重复提交")

// Validate runs mapping + per-row validation without writing anything.
func (s *Service) Validate(c *gin.Context, body WizardBody) (*ValidateResult, error) {
	kind, err := normalizeKind(body.Kind)
	if err != nil {
		return nil, err
	}
	body.Kind = kind
	if err := body.validateShape(); err != nil {
		return nil, err
	}
	if kindNeedsShop(kind) {
		if _, err := s.resolveShop(c, body.ShopID); err != nil {
			return nil, err
		}
	}
	res := &ValidateResult{TotalRows: len(body.Rows)}
	switch kind {
	case KindProduct:
		products, errs := BuildProducts(body.Columns, body.Rows, body.Mapping)
		res.Errors = errs
		res.GroupCount = len(products)
	case KindOrder:
		orders, errs := BuildOrders(body.Columns, body.Rows, body.Mapping)
		res.Errors = errs
		res.GroupCount = len(orders)
	case KindInventory:
		rows, errs := BuildInventoryRows(body.Columns, body.Rows, body.Mapping)
		res.Errors = errs
		res.GroupCount = len(rows)
	case KindSource:
		rows, errs := BuildSourceRows(body.Columns, body.Rows, body.Mapping)
		res.Errors = errs
		res.GroupCount = len(rows)
	case KindPayment:
		rows, errs := BuildPaymentRows(body.Columns, body.Rows, body.Mapping)
		res.Errors = errs
		res.GroupCount = len(rows)
	}
	if res.Errors == nil {
		res.Errors = []RowError{}
	}
	res.ErrorRows = len(res.Errors)
	res.ValidRows = res.TotalRows - res.ErrorRows
	return res, nil
}

// Commit imports validated rows. Batch idempotency: one (tenant, kind,
// batchKey) commits once; re-uploading the same file replays the summary.
func (s *Service) Commit(c *gin.Context, body WizardBody, adminID *uuid.UUID) (*CommitResult, error) {
	kind, err := normalizeKind(body.Kind)
	if err != nil {
		return nil, err
	}
	body.Kind = kind
	if err := body.validateShape(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.FileHash) == "" {
		return nil, fmt.Errorf("fileHash is required")
	}
	var shopID *uuid.UUID
	if kindNeedsShop(kind) {
		shopID, err = s.resolveShop(c, body.ShopID)
		if err != nil {
			return nil, err
		}
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if existing, err := s.findJob(c, tid, kind, body.FileHash); err != nil {
		return nil, err
	} else if existing != nil {
		return replayResult(existing), nil
	}
	pKey := progressKey(tid, kind, body.FileHash)
	if !commits.begin(pKey, len(body.Rows)) {
		return nil, errCommitInFlight
	}
	defer commits.finish(pKey)

	job := &ImportJob{
		TenantID:     tid,
		Kind:         kind,
		BatchKey:     strings.TrimSpace(body.FileHash),
		ShopID:       shopID,
		SourceFormat: normalizeSourceFormat(body.SourceFormat),
		FileName:     strings.TrimSpace(body.FileName),
		TotalRows:    len(body.Rows),
		CreatedBy:    adminID,
	}
	var errorRows []ImportJobRow

	appendErrors := func(errs []RowError) {
		for _, e := range errs {
			errorRows = append(errorRows, ImportJobRow{
				RowNumber: e.RowNumber,
				Status:    RowStatusFailed,
				Field:     e.Field,
				Message:   e.Message,
				RawValues: rawRowJSON(body.Columns, body.Rows, e.RowNumber),
			})
			job.FailedRows++
		}
		commits.advance(pKey, len(errs))
	}

	switch kind {
	case KindProduct:
		products, errs := BuildProducts(body.Columns, body.Rows, body.Mapping)
		appendErrors(errs)
		s.commitProducts(c, job, &errorRows, body, products, adminID)
	case KindOrder:
		orders, errs := BuildOrders(body.Columns, body.Rows, body.Mapping)
		appendErrors(errs)
		s.commitOrders(c, job, &errorRows, body, orders, adminID)
	case KindInventory:
		rows, errs := BuildInventoryRows(body.Columns, body.Rows, body.Mapping)
		appendErrors(errs)
		s.commitInventory(c, job, &errorRows, body, rows, adminID)
	case KindSource:
		rows, errs := BuildSourceRows(body.Columns, body.Rows, body.Mapping)
		appendErrors(errs)
		s.commitSources(c, job, &errorRows, body, rows, adminID)
	case KindPayment:
		rows, errs := BuildPaymentRows(body.Columns, body.Rows, body.Mapping)
		appendErrors(errs)
		s.commitPayments(c, job, &errorRows, body, rows, adminID)
	}

	switch {
	case job.SuccessRows == 0 && job.FailedRows > 0:
		job.Status = JobStatusFailed
	case job.FailedRows > 0 || job.DuplicateRows > 0:
		job.Status = JobStatusPartialSuccess
	default:
		job.Status = JobStatusSuccess
	}

	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		for i := range errorRows {
			errorRows[i].JobID = job.ID
		}
		if len(errorRows) > 0 {
			if err := tx.CreateInBatches(errorRows, 200).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// Unique-violation race: another commit of the same batch won.
		if existing, ferr := s.findJob(c, tid, kind, body.FileHash); ferr == nil && existing != nil {
			return replayResult(existing), nil
		}
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "migration.import",
			Resource:    "import_job",
			ResourceID:  job.ID.String(),
			Status:      "success",
			Message: fmt.Sprintf("kind=%s source=%s total=%d success=%d failed=%d duplicate=%d",
				job.Kind, job.SourceFormat, job.TotalRows, job.SuccessRows, job.FailedRows, job.DuplicateRows),
		})
	}
	return &CommitResult{
		JobID:         job.ID,
		Status:        job.Status,
		TotalRows:     job.TotalRows,
		SuccessRows:   job.SuccessRows,
		FailedRows:    job.FailedRows,
		DuplicateRows: job.DuplicateRows,
	}, nil
}

func replayResult(job *ImportJob) *CommitResult {
	return &CommitResult{
		JobID:         job.ID,
		Status:        job.Status,
		TotalRows:     job.TotalRows,
		SuccessRows:   job.SuccessRows,
		FailedRows:    job.FailedRows,
		DuplicateRows: job.DuplicateRows,
		Replayed:      true,
	}
}

func (s *Service) findJob(c *gin.Context, tenantID int64, kind, batchKey string) (*ImportJob, error) {
	var job ImportJob
	err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND kind = ? AND batch_key = ?", tenantID, kind, strings.TrimSpace(batchKey)).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func rawRowJSON(columns []string, rows [][]string, rowNumber int) datatypes.JSON {
	idx := rowNumber - 1
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	m := map[string]string{}
	for i, col := range columns {
		name := strings.TrimSpace(col)
		if name == "" {
			name = fmt.Sprintf("列%d", i+1)
		}
		if i < len(rows[idx]) {
			m[name] = rows[idx][i]
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

func (s *Service) commitProducts(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, products []ProductInput, adminID *uuid.UUID) {
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	// Duplicate detection runs in bulk: one query per row would dominate the
	// commit time on 10k-row files.
	var existing map[string]bool
	if tenantErr == nil {
		codes := make([]string, 0, len(body.Rows))
		for _, p := range products {
			for _, sku := range p.SKUs {
				codes = append(codes, sku.SKUCode)
			}
		}
		existing, tenantErr = s.existingSKUCodes(c, tid, codes)
	}
	pKey := progressKey(tid, job.Kind, job.BatchKey)
	for _, p := range products {
		if tenantErr != nil {
			rowNos := make([]int, 0, len(p.SKUs))
			for _, sku := range p.SKUs {
				rowNos = append(rowNos, sku.RowNumber)
			}
			s.markRows(job, errorRows, body, rowNos, RowStatusFailed, FSKUCode, tenantErr.Error())
			commits.advance(pKey, len(p.SKUs))
			continue
		}
		var fresh []SKUInput
		for _, sku := range p.SKUs {
			if sku.SKUCode != "" && existing[sku.SKUCode] {
				s.markRows(job, errorRows, body, []int{sku.RowNumber}, RowStatusDuplicate, FSKUCode,
					fmt.Sprintf("SKU「%s」已存在，跳过", sku.SKUCode))
				continue
			}
			fresh = append(fresh, sku)
		}
		if len(fresh) == 0 {
			commits.advance(pKey, len(p.SKUs))
			continue
		}
		detail, err := s.Products.Create(c, product.CreateBody{
			Source:      "migration",
			Title:       p.Title,
			Description: p.Description,
			SourceURL:   p.SourceURL,
			Currency:    p.Currency,
			ShopID:      body.ShopID,
		}, adminID)
		if err != nil {
			rowNos := make([]int, 0, len(fresh))
			for _, sku := range fresh {
				rowNos = append(rowNos, sku.RowNumber)
			}
			s.markRows(job, errorRows, body, rowNos, RowStatusFailed, FTitle, err.Error())
			commits.advance(pKey, len(p.SKUs))
			continue
		}
		for _, sku := range fresh {
			_, err := s.Products.CreateSKU(c, detail.ID, product.SKUBody{
				SKUCode:   sku.SKUCode,
				SKUName:   sku.SKUName,
				Price:     sku.Price,
				CostPrice: sku.CostPrice,
				Stock:     sku.Stock,
				ImageURL:  sku.ImageURL,
			}, adminID)
			if err != nil {
				s.markRows(job, errorRows, body, []int{sku.RowNumber}, RowStatusFailed, FSKUCode, err.Error())
				continue
			}
			job.SuccessRows++
		}
		commits.advance(pKey, len(p.SKUs))
	}
}

func (s *Service) commitOrders(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, orders []OrderInput, adminID *uuid.UUID) {
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	// Duplicate detection is per tenant and in bulk: a global lookup would
	// report another tenant's order number as an existing one, and one query
	// per row would dominate the commit time on 10k-row files.
	var existing map[string]bool
	if tenantErr == nil {
		nos := make([]string, 0, len(orders))
		for _, oi := range orders {
			nos = append(nos, oi.OrderNo)
		}
		existing, tenantErr = s.existingOrderNos(c, tid, nos)
	}
	pKey := progressKey(tid, job.Kind, job.BatchKey)
	for _, oi := range orders {
		if tenantErr != nil {
			s.markRows(job, errorRows, body, oi.RowNumbers, RowStatusFailed, FOrderNo, tenantErr.Error())
			commits.advance(pKey, len(oi.RowNumbers))
			continue
		}
		if existing[oi.OrderNo] {
			s.markRows(job, errorRows, body, oi.RowNumbers, RowStatusDuplicate, FOrderNo,
				fmt.Sprintf("订单号「%s」已存在，跳过", oi.OrderNo))
			commits.advance(pKey, len(oi.RowNumbers))
			continue
		}
		cb := oi.ToCreateBody(normalizeSourceFormat(body.SourceFormat))
		cb.ShopID = job.ShopID
		if _, err := s.Orders.Create(c, cb, adminID); err != nil {
			s.markRows(job, errorRows, body, oi.RowNumbers, RowStatusFailed, "", err.Error())
			commits.advance(pKey, len(oi.RowNumbers))
			continue
		}
		job.SuccessRows += len(oi.RowNumbers)
		commits.advance(pKey, len(oi.RowNumbers))
	}
}

func (s *Service) markRows(job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, rowNos []int, status, field, message string) {
	if len(message) > 500 {
		message = message[:500]
	}
	for _, rn := range rowNos {
		*errorRows = append(*errorRows, ImportJobRow{
			RowNumber: rn,
			Status:    status,
			Field:     field,
			Message:   message,
			RawValues: rawRowJSON(body.Columns, body.Rows, rn),
		})
		if status == RowStatusDuplicate {
			job.DuplicateRows++
		} else {
			job.FailedRows++
		}
	}
}

// ListJobs returns tenant + store scoped import history (newest first).
// Jobs always carry the target shop, so non-admin principals only see the
// history of shops they are granted (same semantics as order lists).
func (s *Service) ListJobs(c *gin.Context, kind string, page, pageSize int) ([]JobDTO, int64, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, 0, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&ImportJob{}).Where("tenant_id = ?", tid)
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, 0, err
	}
	if k := strings.TrimSpace(kind); k != "" {
		if _, err := normalizeKind(k); err != nil {
			return nil, 0, err
		}
		tx = tx.Where("kind = ?", k)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var jobs []ImportJob
	if err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	out := make([]JobDTO, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, JobDTO{ImportJob: j, ErrorRowCount: j.FailedRows + j.DuplicateRows})
	}
	return out, total, nil
}

// GetJob loads one job with its error rows, enforcing tenant + store scope:
// a job outside the caller's store scope is indistinguishable from a missing
// one (404, no existence leak).
func (s *Service) GetJob(c *gin.Context, id uuid.UUID) (*ImportJob, []ImportJobRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, nil, err
	}
	var job ImportJob
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).First(&job, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, job.ShopID); err != nil {
		return nil, nil, err
	}
	var rows []ImportJobRow
	if err := s.DB.WithContext(c.Request.Context()).
		Where("job_id = ?", job.ID).Order("row_number ASC").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	return &job, rows, nil
}
