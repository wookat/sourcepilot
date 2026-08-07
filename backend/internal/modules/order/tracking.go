package order

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/providers/tracking"
)

// RefreshShipmentTracking asks the active TrackingProvider for the latest
// waybill status and applies it through the normal shipment update flow (so
// in_transit / delivered keep advancing the order exactly as manual edits
// do). With the manual provider this is a no-op returning the current row.
func (s *Service) RefreshShipmentTracking(c *gin.Context, orderID, shipmentID uuid.UUID, adminID *uuid.UUID) (*OrderShipment, error) {
	if _, err := s.findOrderOperable(c, orderID); err != nil {
		return nil, err
	}
	var row OrderShipment
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND order_id = ?", shipmentID, orderID).Error; err != nil {
		return nil, err
	}
	provider := tracking.Default()
	if !provider.SupportsFetch() {
		return &row, nil
	}
	result, err := provider.Fetch(c.Request.Context(), row.CarrierCode, row.TrackingNo)
	if err != nil {
		if errors.Is(err, tracking.ErrManualOnly) {
			return &row, nil
		}
		return nil, err
	}
	st := strings.TrimSpace(result.Status)
	if st == "" || st == row.Status {
		return &row, nil
	}
	return s.PatchShipment(c, orderID, shipmentID, OrderShipmentInput{
		Carrier:     row.Carrier,
		TrackingNo:  row.TrackingNo,
		TrackingURL: row.TrackingURL,
		Status:      st,
	}, adminID)
}
