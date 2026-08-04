package order

import "strings"

// isOrderNoUniqueViolation reports whether err is the tenant-scoped order_no
// unique index violation (idx_orders_tenant_order_no), so callers can surface
// a readable business message instead of the raw SQL error.
func isOrderNoUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// PostgreSQL names the index; SQLite (dev) names the columns.
	if !strings.Contains(msg, "idx_orders_tenant_order_no") &&
		!strings.Contains(msg, "orders.tenant_id, orders.order_no") {
		return false
	}
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "constraint failed") ||
		strings.Contains(msg, "sqlstate 23505")
}

func validOrderStatus(s string) bool {
	switch s {
	case StatusPending, StatusPaid, StatusProcessing, StatusShipped, StatusDelivered, StatusCancelled, StatusRefunded, StatusClosed:
		return true
	default:
		return false
	}
}

func validPaymentStatus(s string) bool {
	switch s {
	case PaymentUnpaid, PaymentPaid, PaymentPartiallyRefunded, PaymentRefunded:
		return true
	default:
		return false
	}
}

func validFulfillmentStatus(s string) bool {
	switch s {
	case FulfillmentUnfulfilled, FulfillmentPartial, FulfillmentFulfilled, FulfillmentReturned:
		return true
	default:
		return false
	}
}

func validShipmentStatus(s string) bool {
	switch s {
	case ShipmentPending, ShipmentShipped, ShipmentInTransit, ShipmentDelivered, ShipmentException, ShipmentReturned:
		return true
	default:
		return false
	}
}
