package migrationimport

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// PaymentInput is one validated payment (回款) import row.
type PaymentInput struct {
	RowNumber  int
	OrderNo    string
	Amount     float64
	Currency   string
	FeeAmount  float64
	ReceivedAt time.Time
	Channel    string
	Remark     string
}

// BuildPaymentRows maps and validates payment import rows (one file row =
// one payment record; duplicates within the file are per order+amount+date).
func BuildPaymentRows(columns []string, rows [][]string, mapping map[string]int) ([]PaymentInput, []RowError) {
	mapping = normalizedMapping(KindPayment, mapping)
	var errs []RowError
	var out []PaymentInput
	seen := map[string]int{}
	for i, row := range rows {
		rowNo := i + 1
		orderNo := cellAt(row, mapping[FOrderNo])
		if orderNo == "" {
			errs = append(errs, RowError{rowNo, FOrderNo, "订单号不能为空"})
			continue
		}
		amount, err := parseFloatCell(cellAt(row, mapping[FPaymentAmount]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FPaymentAmount, "回款金额" + err.Error()})
			continue
		}
		if amount == nil || *amount <= 0 {
			errs = append(errs, RowError{rowNo, FPaymentAmount, "回款金额必须大于 0"})
			continue
		}
		fee, err := parseFloatCell(cellAt(row, mapping[FFeeAmount]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FFeeAmount, "手续费" + err.Error()})
			continue
		}
		if fee != nil && *fee > *amount {
			errs = append(errs, RowError{rowNo, FFeeAmount, "手续费不能超过回款金额"})
			continue
		}
		receivedAt, err := parseTimeCell(cellAt(row, mapping[FReceivedAt]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FReceivedAt, "回款日期" + err.Error()})
			continue
		}
		if receivedAt == nil {
			errs = append(errs, RowError{rowNo, FReceivedAt, "回款日期不能为空"})
			continue
		}
		cur := strings.ToUpper(cellAt(row, mapping[FCurrency]))
		if cur != "" && !currencyOK(cur) {
			errs = append(errs, RowError{rowNo, FCurrency, "币种代码无效（如 CNY、USD）"})
			continue
		}
		key := fmt.Sprintf("%s\x00%.4f\x00%s\x00%s", orderNo, *amount, cur, receivedAt.Format("2006-01-02"))
		if prev, dup := seen[key]; dup {
			errs = append(errs, RowError{rowNo, FOrderNo, fmt.Sprintf("与第 %d 行重复（同订单+金额+日期）", prev)})
			continue
		}
		seen[key] = rowNo
		out = append(out, PaymentInput{
			RowNumber:  rowNo,
			OrderNo:    orderNo,
			Amount:     *amount,
			Currency:   cur,
			FeeAmount:  derefFloat(fee),
			ReceivedAt: *receivedAt,
			Channel:    cellAt(row, mapping[FChannel]),
			Remark:     cellAt(row, mapping[FRemark]),
		})
	}
	return out, errs
}

// commitPayments imports payment rows: each row must match an existing
// tenant order visible + operable for the caller; identical existing records
// (same order, amount, currency, received day) are skipped as duplicates.
func (s *Service) commitPayments(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, inputs []PaymentInput, adminID *uuid.UUID) {
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	if s.Finance == nil {
		tenantErr = fmt.Errorf("回款导入未启用（财务模块未初始化）")
	}
	principal, perr := adminperm.LoadPrincipal(c, s.DB)
	if tenantErr == nil && perr != nil {
		tenantErr = perr
	}
	var ordersByNo map[string]order.Order
	if tenantErr == nil {
		nos := make([]string, 0, len(inputs))
		for _, in := range inputs {
			nos = append(nos, in.OrderNo)
		}
		ordersByNo, tenantErr = s.ordersByNo(c, tid, nos)
	}
	pKey := progressKey(tid, job.Kind, job.BatchKey)
	for _, in := range inputs {
		if tenantErr != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FOrderNo, tenantErr.Error())
			commits.advance(pKey, 1)
			continue
		}
		o, ok := ordersByNo[in.OrderNo]
		if !ok {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FOrderNo,
				fmt.Sprintf("订单号「%s」不存在", in.OrderNo))
			commits.advance(pKey, 1)
			continue
		}
		if o.ShopID != nil && principal != nil && !principal.IsAdmin() {
			if !principal.CanViewStore(*o.ShopID) {
				s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FOrderNo,
					fmt.Sprintf("订单号「%s」不存在", in.OrderNo))
				commits.advance(pKey, 1)
				continue
			}
			if !principal.CanOperateStore(*o.ShopID) {
				s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FOrderNo, "当前账号无该店铺的操作权限")
				commits.advance(pKey, 1)
				continue
			}
		}
		cur := in.Currency
		if cur == "" {
			cur = o.Currency
		}
		dup, err := s.Finance.FindDuplicatePayment(c.Request.Context(), tid, o.ID, in.Amount, cur, in.ReceivedAt)
		if err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FOrderNo, err.Error())
			commits.advance(pKey, 1)
			continue
		}
		if dup {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusDuplicate, FOrderNo,
				fmt.Sprintf("订单「%s」已存在相同金额和日期的回款记录，跳过", in.OrderNo))
			commits.advance(pKey, 1)
			continue
		}
		if _, err := s.Finance.CreatePaymentForImport(c.Request.Context(), &o, finance.PaymentBody{
			Amount:     in.Amount,
			Currency:   cur,
			FeeAmount:  in.FeeAmount,
			ReceivedAt: in.ReceivedAt.Format("2006-01-02"),
			Channel:    in.Channel,
			Remark:     in.Remark,
		}, adminID); err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, "", err.Error())
			commits.advance(pKey, 1)
			continue
		}
		job.SuccessRows++
		commits.advance(pKey, 1)
	}
}

// ordersByNo bulk-loads tenant orders by order number.
func (s *Service) ordersByNo(c *gin.Context, tenantID int64, nos []string) (map[string]order.Order, error) {
	out := map[string]order.Order{}
	if len(nos) == 0 {
		return out, nil
	}
	var rows []order.Order
	if err := s.DB.WithContext(c.Request.Context()).
		Select("id, order_no, currency, total_amount, shop_id, tenant_id").
		Where("tenant_id = ? AND order_no IN ?", tenantID, nos).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, o := range rows {
		out[o.OrderNo] = o
	}
	return out, nil
}
