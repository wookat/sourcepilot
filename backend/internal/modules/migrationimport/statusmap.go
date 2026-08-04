package migrationimport

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// OrderStatusMapping is the resolved internal status triple for one source status.
type OrderStatusMapping struct {
	Status            string
	PaymentStatus     string
	FulfillmentStatus string
}

// orderStatusAliases maps normalized Dianxiaomi / Mabang / generic order
// statuses onto the internal enums.
var orderStatusAliases = map[string]OrderStatusMapping{
	// unpaid
	"未付款":     {order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"待付款":     {order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"待支付":     {order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"unpaid":  {order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"pending": {order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	// paid, not yet processed
	"已付款":  {order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"已支付":  {order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"paid": {order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled},
	// processing (Dianxiaomi pipeline states)
	"待处理":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"待审核":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"待打单":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"已打单":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"配货中":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"处理中":        {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	"processing": {order.StatusProcessing, order.PaymentPaid, order.FulfillmentUnfulfilled},
	// shipped
	"已发货":     {order.StatusShipped, order.PaymentPaid, order.FulfillmentFulfilled},
	"运输中":     {order.StatusShipped, order.PaymentPaid, order.FulfillmentFulfilled},
	"shipped": {order.StatusShipped, order.PaymentPaid, order.FulfillmentFulfilled},
	// delivered / finished
	"已完成":       {order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
	"已签收":       {order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
	"完成":        {order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
	"delivered": {order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
	"finished":  {order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
	// cancelled
	"已作废":       {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"作废":        {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"已取消":       {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"取消":        {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"cancelled": {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	"canceled":  {order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled},
	// refunded
	"已退款":      {order.StatusRefunded, order.PaymentRefunded, order.FulfillmentUnfulfilled},
	"退款":       {order.StatusRefunded, order.PaymentRefunded, order.FulfillmentUnfulfilled},
	"refunded": {order.StatusRefunded, order.PaymentRefunded, order.FulfillmentUnfulfilled},
}

// MapOrderStatus resolves a source order status; ok=false when unknown.
func MapOrderStatus(raw string) (OrderStatusMapping, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	m, ok := orderStatusAliases[key]
	return m, ok
}
