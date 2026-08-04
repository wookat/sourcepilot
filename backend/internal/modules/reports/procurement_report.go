package reports

import (
	"context"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Lead-time distribution buckets (下单→签收 days, inclusive bounds).
var leadTimeBuckets = []struct {
	Label string
	Min   int
	Max   int // -1 = unbounded
}{
	{"0-3 天", 0, 3},
	{"4-7 天", 4, 7},
	{"8-15 天", 8, 15},
	{"16 天以上", 16, -1},
}

const procurementMaxSuppliers = 100

// LeadTimeBucket is one 下单→签收 duration bucket.
type LeadTimeBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// SupplierAgg is one supplier's aggregated purchasing in the range.
type SupplierAgg struct {
	SupplierID      string   `json:"supplierId"`
	SupplierName    string   `json:"supplierName"`
	POCount         int64    `json:"poCount"`
	Amount          float64  `json:"amount"`
	DeliveredCount  int64    `json:"deliveredCount"`
	AvgLeadTimeDays *float64 `json:"avgLeadTimeDays,omitempty"`
}

// ProcurementDaily is one day's purchasing volume.
type ProcurementDaily struct {
	Date    string  `json:"date"`
	POCount int64   `json:"poCount"`
	Amount  float64 `json:"amount"`
}

// ProcurementSummary aggregates all purchase orders in the range. Amounts
// are CNY (purchase orders are sourced from 1688). Voided orders are
// excluded from amounts; cancelled orders are counted separately.
type ProcurementSummary struct {
	POCount         int64    `json:"poCount"`
	TotalAmount     float64  `json:"totalAmount"`
	InTransitCount  int64    `json:"inTransitCount"`
	DeliveredCount  int64    `json:"deliveredCount"`
	CancelledCount  int64    `json:"cancelledCount"`
	AvgLeadTimeDays *float64 `json:"avgLeadTimeDays,omitempty"`
	LeadTimeSamples int64    `json:"leadTimeSamples"`
}

// ProcurementReportDTO is GET /reports/procurement.
type ProcurementReportDTO struct {
	GeneratedAt string             `json:"generatedAt"`
	StartDate   string             `json:"startDate"`
	EndDate     string             `json:"endDate"`
	Currency    string             `json:"currency"`
	Summary     ProcurementSummary `json:"summary"`
	Daily       []ProcurementDaily `json:"daily"`
	LeadTime    []LeadTimeBucket   `json:"leadTime"`
	Suppliers   []SupplierAgg      `json:"suppliers"`
}

// inTransitStatuses: confirmed orders not yet received at the warehouse.
var inTransitStatuses = map[string]bool{
	procurement.StatusPlacing: true,
	procurement.StatusPlaced:  true,
	procurement.StatusPaid:    true,
	procurement.StatusShipped: true,
}

// ProcurementReport aggregates purchase orders created in the range for the
// current tenant. Store-scoped principals only see purchase orders linked
// (via their items' sales orders) to authorized shops, mirroring POInScope.
func (s *Service) ProcurementReport(c *gin.Context, r DateRange) (*ProcurementReportDTO, error) {
	ctx := c.Request.Context()

	tx := s.DB.WithContext(ctx).Model(&procurement.PurchaseOrder{}).
		Where("created_at >= ? AND created_at < ?", r.Start, r.End)
	tx, tenantID, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, err
	}
	_ = tenantID
	allowed, err := allowedShopIDs(c, s)
	if err != nil {
		return nil, err
	}
	if allowed != nil {
		if len(allowed) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where(`EXISTS (
SELECT 1 FROM purchase_order_items poi
JOIN orders so ON so.id = poi.sales_order_id
WHERE poi.purchase_order_id = purchase_orders.id AND so.shop_id IN ?)`, allowed)
		}
	}

	var pos []procurement.PurchaseOrder
	if err := tx.Select("id, supplier_id, supplier_name, status, total_amount, currency, created_at").Find(&pos).Error; err != nil {
		return nil, err
	}

	deliveredAt, err := s.deliveredTimes(ctx, pos)
	if err != nil {
		return nil, err
	}

	out := &ProcurementReportDTO{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		StartDate:   r.StartDate(),
		EndDate:     r.EndDate(),
		Currency:    "CNY",
		Daily:       []ProcurementDaily{},
		Suppliers:   []SupplierAgg{},
	}

	buckets := make([]int64, len(leadTimeBuckets))
	dailyCount := map[string]int64{}
	dailyAmount := map[string]float64{}
	type supAcc struct {
		name      string
		count     int64
		amount    float64
		delivered int64
		leadSum   float64
		leadN     int64
	}
	sups := map[uuid.UUID]*supAcc{}
	supOrder := []uuid.UUID{}
	var leadSum float64
	loc := time.Now().Location()

	for _, po := range pos {
		if po.Status == procurement.StatusVoided {
			continue
		}
		out.Summary.POCount++
		day := po.CreatedAt.In(loc).Format("2006-01-02")
		dailyCount[day]++
		switch po.Status {
		case procurement.StatusCancelled:
			out.Summary.CancelledCount++
		default:
			out.Summary.TotalAmount += po.TotalAmount
			dailyAmount[day] += po.TotalAmount
		}
		if inTransitStatuses[po.Status] {
			out.Summary.InTransitCount++
		}

		sa := sups[po.SupplierID]
		if sa == nil {
			sa = &supAcc{name: po.SupplierName}
			sups[po.SupplierID] = sa
			supOrder = append(supOrder, po.SupplierID)
		}
		sa.count++
		if po.Status != procurement.StatusCancelled {
			sa.amount += po.TotalAmount
		}

		if po.Status == procurement.StatusDelivered {
			out.Summary.DeliveredCount++
			sa.delivered++
			if dt, ok := deliveredAt[po.ID]; ok && !dt.Before(po.CreatedAt) {
				days := dt.Sub(po.CreatedAt).Hours() / 24
				leadSum += days
				out.Summary.LeadTimeSamples++
				sa.leadSum += days
				sa.leadN++
				d := int(days)
				for i, b := range leadTimeBuckets {
					if d >= b.Min && (b.Max < 0 || d <= b.Max) {
						buckets[i]++
						break
					}
				}
			}
		}
	}
	out.Summary.TotalAmount = round2(out.Summary.TotalAmount)
	if out.Summary.LeadTimeSamples > 0 {
		v := round2(leadSum / float64(out.Summary.LeadTimeSamples))
		out.Summary.AvgLeadTimeDays = &v
	}

	for i, b := range leadTimeBuckets {
		out.LeadTime = append(out.LeadTime, LeadTimeBucket{Label: b.Label, Count: buckets[i]})
	}
	for i := 0; i < r.Days; i++ {
		d := r.Start.AddDate(0, 0, i).Format("2006-01-02")
		out.Daily = append(out.Daily, ProcurementDaily{Date: d, POCount: dailyCount[d], Amount: round2(dailyAmount[d])})
	}
	for _, id := range supOrder {
		sa := sups[id]
		agg := SupplierAgg{
			SupplierID:     id.String(),
			SupplierName:   sa.name,
			POCount:        sa.count,
			Amount:         round2(sa.amount),
			DeliveredCount: sa.delivered,
		}
		if sa.leadN > 0 {
			v := round2(sa.leadSum / float64(sa.leadN))
			agg.AvgLeadTimeDays = &v
		}
		out.Suppliers = append(out.Suppliers, agg)
	}
	sort.SliceStable(out.Suppliers, func(i, j int) bool { return out.Suppliers[i].Amount > out.Suppliers[j].Amount })
	if len(out.Suppliers) > procurementMaxSuppliers {
		out.Suppliers = out.Suppliers[:procurementMaxSuppliers]
	}
	return out, nil
}

// deliveredTimes resolves each delivered purchase order's 签收 timestamp from
// the earliest delivered transition event.
func (s *Service) deliveredTimes(ctx context.Context, pos []procurement.PurchaseOrder) (map[uuid.UUID]time.Time, error) {
	ids := []uuid.UUID{}
	for _, po := range pos {
		if po.Status == procurement.StatusDelivered {
			ids = append(ids, po.ID)
		}
	}
	out := map[uuid.UUID]time.Time{}
	if len(ids) == 0 {
		return out, nil
	}
	const chunk = 500
	for i := 0; i < len(ids); i += chunk {
		j := i + chunk
		if j > len(ids) {
			j = len(ids)
		}
		type evRow struct {
			PurchaseOrderID uuid.UUID `gorm:"column:purchase_order_id"`
			At              string    `gorm:"column:at"`
		}
		var rows []evRow
		if err := s.DB.WithContext(ctx).Model(&procurement.PurchaseOrderEvent{}).
			Select("purchase_order_id, MIN(created_at) AS at").
			Where("purchase_order_id IN ? AND to_status = ?", ids[i:j], procurement.StatusDelivered).
			Group("purchase_order_id").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, rrow := range rows {
			if t, ok := parseDBTime(rrow.At); ok {
				out[rrow.PurchaseOrderID] = t
			}
		}
	}
	return out, nil
}

// allowedShopIDs returns nil for unrestricted principals (admin), otherwise
// the granted shop IDs (empty slice = none visible).
func allowedShopIDs(c *gin.Context, s *Service) ([]uuid.UUID, error) {
	p, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil || p == nil {
		return nil, nil
	}
	return p.AllowedStoreIDs(), nil
}
