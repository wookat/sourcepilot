package procurement

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/providers/trade"
	"gorm.io/gorm"
)

// MCP write adapter for purchase orders (R180 W2 白名单动作 mark-placed /
// 物流运单号回填). Methods are tenant-scoped by explicit tenant id (MCP tokens
// carry no admin principal / store grants) and accept an external *gorm.DB so
// the caller can run the mutation and its audit row in one transaction.
// Cross-tenant / missing purchase orders surface as ErrPONotFoundInTenant
// (404 semantics, no existence oracle).

// ErrPONotFoundInTenant is returned when a purchase order id does not resolve
// inside the tenant (missing or cross-tenant — indistinguishable on purpose).
var ErrPONotFoundInTenant = errors.New("采购单不存在")

// FindPOInTenant resolves one purchase order by id within the tenant.
func (s *Service) FindPOInTenant(ctx context.Context, db *gorm.DB, tenantID int64, id uuid.UUID) (*PurchaseOrder, error) {
	if s == nil || db == nil {
		return nil, errors.New("procurement: no db")
	}
	var po PurchaseOrder
	if err := db.WithContext(ctx).
		First(&po, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPONotFoundInTenant
		}
		return nil, err
	}
	return &po, nil
}

// MarkPlacedInTenantTx applies the placing → placed transition inside the
// caller's transaction (same semantics as MarkPlaced, without per-line actual
// prices — the MCP surface only backfills the external order id).
func (s *Service) MarkPlacedInTenantTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID, externalOrderID string) (*PurchaseOrder, error) {
	ext := strings.TrimSpace(externalOrderID)
	if ext == "" {
		return nil, errors.New("externalOrderId 不能为空")
	}
	if _, err := s.FindPOInTenant(ctx, tx, tenantID, id); err != nil {
		return nil, err
	}
	po, err := s.transitionTx(tx.WithContext(ctx), id, StatusPlaced, EventSourceAPI,
		map[string]any{"externalOrderId": ext, "via": "mcp"}, markPlacedMutate(ext, nil))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPONotFoundInTenant
		}
		return nil, err
	}
	return po, nil
}

// FillLogisticsInTenantTx applies the paid → shipped transition (tracking
// number backfill) inside the caller's transaction.
func (s *Service) FillLogisticsInTenantTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID, trackingNo, carrier string) (*PurchaseOrder, error) {
	tn := strings.TrimSpace(trackingNo)
	if tn == "" {
		return nil, errors.New("trackingNo 不能为空")
	}
	if _, err := s.FindPOInTenant(ctx, tx, tenantID, id); err != nil {
		return nil, err
	}
	po, err := s.transitionTx(tx.WithContext(ctx), id, StatusShipped, EventSourceAPI,
		map[string]any{"trackingNo": tn, "via": "mcp"}, fillLogisticsMutate(tn, strings.TrimSpace(carrier)))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPONotFoundInTenant
		}
		return nil, err
	}
	return po, nil
}

// MarkPaidInTenantTx applies the placed → paid transition (manual payment
// backfill) inside the caller's transaction. The MCP surface never moves
// real money: it only records that the operator paid the 1688 order by hand.
// Amount/currency validation against the purchase order and the tenant
// ceilings are enforced by the MCP tool layer before this runs.
func (s *Service) MarkPaidInTenantTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID, payChannel string) (*PurchaseOrder, error) {
	if _, err := s.FindPOInTenant(ctx, tx, tenantID, id); err != nil {
		return nil, err
	}
	channel := defaultStr(strings.TrimSpace(payChannel), "manual")
	po, err := s.transitionTx(tx.WithContext(ctx), id, StatusPaid, EventSourceAPI,
		map[string]any{"payChannel": channel, "via": "mcp"}, func(tx *gorm.DB, po *PurchaseOrder) error {
			now := time.Now().UTC()
			po.PayStatus = PayStatusPaid
			po.PayChannel = channel
			po.PaidAt = &now
			return nil
		})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPONotFoundInTenant
		}
		return nil, err
	}
	return po, nil
}

// AfterMarkPaidCommitted runs the best-effort post-commit side effects of a
// mark-paid (mock provider pay advance + operation log), mirroring the admin
// route after the MCP write pipeline commits.
func (s *Service) AfterMarkPaidCommitted(ctx context.Context, po *PurchaseOrder) {
	if s == nil || po == nil {
		return
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok && po.ExternalOrderID != "" {
		_ = mock.AdvancePay(po.ExternalOrderID)
	}
	s.logOp(ctx, nil, "procurement.mark_paid", po.ID.String(), "via=mcp")
}

// AfterMarkPlacedCommitted runs the best-effort post-commit side effects of a
// mark-placed (mock provider registration + operation log), mirroring the
// admin route after the MCP write pipeline commits.
func (s *Service) AfterMarkPlacedCommitted(ctx context.Context, po *PurchaseOrder) {
	if s == nil || po == nil {
		return
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok && po.ExternalOrderID != "" {
		mock.RegisterManualOrder(po.ExternalOrderID, po.TotalAmount)
	}
	s.logOp(ctx, nil, "procurement.mark_placed", po.ID.String(), po.ExternalOrderID+" via=mcp")
}

// AfterFillLogisticsCommitted runs the best-effort post-commit side effects
// of a logistics backfill (mock provider advance, operation log and the order
// automation event), mirroring the admin route after the MCP write pipeline
// commits.
func (s *Service) AfterFillLogisticsCommitted(ctx context.Context, po *PurchaseOrder, trackingNo, carrier string) {
	if s == nil || po == nil {
		return
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok && po.ExternalOrderID != "" {
		_ = mock.AdvanceShip(po.ExternalOrderID, carrier, trackingNo)
	}
	s.logOp(ctx, nil, "procurement.fill_logistics", po.ID.String(), trackingNo+" via=mcp")
	s.fireOrderEvents(ctx, po, order.AutomationEventLogisticsCollected)
}
