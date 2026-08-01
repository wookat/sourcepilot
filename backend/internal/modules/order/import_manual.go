package order

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// Import row statuses.
const (
	ImportRowCreated   = "created"
	ImportRowDuplicate = "skipped_duplicate"
	ImportRowFailed    = "failed"
)

// ImportBody POST /orders/import — batch create manual orders.
type ImportBody struct {
	Orders    []CreateBody `json:"orders"`
	MatchSKUs bool         `json:"matchSkus"`
}

// ImportRowResult reports the outcome for one order in the batch.
type ImportRowResult struct {
	OrderNo      string     `json:"orderNo"`
	Status       string     `json:"status"`
	OrderID      *uuid.UUID `json:"orderId,omitempty"`
	Error        string     `json:"error,omitempty"`
	ItemsTotal   int        `json:"itemsTotal"`
	ItemsMatched int        `json:"itemsMatched"`
}

// ImportSummary aggregates one import run.
type ImportSummary struct {
	Total     int               `json:"total"`
	Created   int               `json:"created"`
	Duplicate int               `json:"duplicate"`
	Failed    int               `json:"failed"`
	Results   []ImportRowResult `json:"results"`
}

const maxImportOrders = 200

// ImportOrders creates a batch of manual orders: per-row outcome, duplicate
// order numbers (in DB or within the batch) are skipped, optional SKU
// auto-match after creation. A single row failure does not abort the batch.
func (s *Service) ImportOrders(c *gin.Context, body ImportBody, adminID *uuid.UUID) (*ImportSummary, error) {
	if len(body.Orders) == 0 {
		return nil, fmt.Errorf("orders is required")
	}
	if len(body.Orders) > maxImportOrders {
		return nil, fmt.Errorf("too many orders in one batch (max %d)", maxImportOrders)
	}
	sum := &ImportSummary{Total: len(body.Orders)}
	seen := map[string]bool{}
	for _, ob := range body.Orders {
		orderNo := strings.TrimSpace(ob.OrderNo)
		row := ImportRowResult{OrderNo: orderNo, ItemsTotal: len(ob.Items)}
		if orderNo == "" {
			row.Status = ImportRowFailed
			row.Error = "orderNo is required"
			sum.Failed++
			sum.Results = append(sum.Results, row)
			continue
		}
		if seen[orderNo] {
			row.Status = ImportRowDuplicate
			sum.Duplicate++
			sum.Results = append(sum.Results, row)
			continue
		}
		seen[orderNo] = true
		var cnt int64
		if err := s.DB.WithContext(c.Request.Context()).Model(&Order{}).
			Where("order_no = ?", orderNo).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt > 0 {
			row.Status = ImportRowDuplicate
			sum.Duplicate++
			sum.Results = append(sum.Results, row)
			continue
		}
		out, err := s.Create(c, ob, adminID)
		if err != nil {
			row.Status = ImportRowFailed
			row.Error = err.Error()
			sum.Failed++
			sum.Results = append(sum.Results, row)
			continue
		}
		row.Status = ImportRowCreated
		row.OrderID = &out.ID
		if body.MatchSKUs {
			if ms, err := s.MatchOrderItemsForOrder(c.Request.Context(), out.ID, MatchOrderItemsOptions{
				CreatedBy: adminID,
				Source:    "order_import",
			}); err == nil && ms != nil {
				row.ItemsMatched = ms.Matched
			}
		}
		sum.Created++
		sum.Results = append(sum.Results, row)
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.import",
			Resource:    "order",
			Status:      "success",
			Message:     fmt.Sprintf("total=%d created=%d duplicate=%d failed=%d", sum.Total, sum.Created, sum.Duplicate, sum.Failed),
		})
	}
	return sum, nil
}
