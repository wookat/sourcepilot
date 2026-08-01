package order

import (
	"time"

	"github.com/gin-gonic/gin"
)

// shipmentOrderHint maps a shipment status to the order status / fulfillment
// status the order should be advanced to (forward-only).
func shipmentOrderHint(shipmentStatus string) (orderStatus, fulfillmentStatus string) {
	switch shipmentStatus {
	case ShipmentShipped, ShipmentInTransit:
		return StatusShipped, FulfillmentFulfilled
	case ShipmentDelivered:
		return StatusDelivered, FulfillmentFulfilled
	default:
		return "", ""
	}
}

// fillShipmentTimestamps defaults ShippedAt / DeliveredAt from the shipment
// status when the caller did not provide them.
func fillShipmentTimestamps(row *OrderShipment) {
	if row == nil {
		return
	}
	now := time.Now().UTC()
	switch row.Status {
	case ShipmentShipped, ShipmentInTransit:
		if row.ShippedAt == nil {
			row.ShippedAt = &now
		}
	case ShipmentDelivered:
		if row.ShippedAt == nil {
			row.ShippedAt = &now
		}
		if row.DeliveredAt == nil {
			row.DeliveredAt = &now
		}
	}
}

// advanceOrderOnShipment moves the order lifecycle forward after a shipment
// write. It never regresses the order (rank-guarded) and fills shipped /
// delivered timestamps on the order when absent.
func (s *Service) advanceOrderOnShipment(c *gin.Context, o *Order, shipmentStatus string) error {
	if s == nil || s.DB == nil || o == nil {
		return nil
	}
	targetStatus, targetFulfillment := shipmentOrderHint(shipmentStatus)
	if targetStatus == "" {
		return nil
	}
	curRank := orderLifecycleRank(o.Status, o.PaymentStatus, o.FulfillmentStatus)
	incRank := orderLifecycleRank(targetStatus, o.PaymentStatus, targetFulfillment)
	if incRank <= curRank {
		return nil
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":             targetStatus,
		"fulfillment_status": targetFulfillment,
	}
	if o.ShippedAt == nil {
		updates["shipped_at"] = now
	}
	if targetStatus == StatusDelivered && o.DeliveredAt == nil {
		updates["delivered_at"] = now
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(&Order{}).Where("id = ?", o.ID).Updates(updates).Error; err != nil {
		return err
	}
	o.Status = targetStatus
	o.FulfillmentStatus = targetFulfillment
	if o.ShippedAt == nil {
		o.ShippedAt = &now
	}
	if targetStatus == StatusDelivered && o.DeliveredAt == nil {
		o.DeliveredAt = &now
	}
	return nil
}
