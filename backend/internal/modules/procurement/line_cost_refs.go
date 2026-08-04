package procurement

import (
	"context"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// LineCostRef is one sales-order line's resolved reference procurement unit
// cost (CNY), or the issue that prevents pricing it. Resolution rules are
// identical to cost estimates: primary source SKU mapping current price,
// falling back to the latest captured price history.
type LineCostRef struct {
	UnitCostCNY  *float64
	SupplierName string
	IssueCode    string
	IssueMessage string
}

// ResolveLineCostRefs exposes batched reference line costs to read-side
// consumers (profit reports) without duplicating the resolution logic.
func (s *Service) ResolveLineCostRefs(ctx context.Context, items []order.OrderItem) (map[uuid.UUID]LineCostRef, error) {
	costs, err := s.resolveLineCostBatch(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]LineCostRef, len(costs))
	for id, c := range costs {
		out[id] = LineCostRef{UnitCostCNY: c.unit, SupplierName: c.supplierName, IssueCode: c.issueCode, IssueMessage: c.issueMessage}
	}
	return out, nil
}
