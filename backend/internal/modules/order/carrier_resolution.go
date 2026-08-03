package order

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
)

// resolveShipmentInput links one shipment input to a tenant carrier when a
// carrierCode is supplied: it fills the carrier display name, the tracking
// URL (from the carrier template) and loosely validates the waybill format.
// Free-text carriers without a code keep the legacy behaviour untouched.
func (s *Service) resolveShipmentInput(c *gin.Context, in *OrderShipmentInput) error {
	if in == nil {
		return nil
	}
	code := strings.TrimSpace(in.CarrierCode)
	if code == "" {
		return nil
	}
	if s == nil || s.Carriers == nil {
		return fmt.Errorf("物流商服务不可用")
	}
	cr, err := s.Carriers.ResolveEnabled(c, code)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return fmt.Errorf("物流商不存在或已停用：%s", code)
		}
		return err
	}
	if err := carrier.ValidateTrackingNo(cr.Code, in.TrackingNo); err != nil {
		return err
	}
	in.CarrierCode = cr.Code
	in.CarrierID = &cr.ID
	if strings.TrimSpace(in.Carrier) == "" {
		in.Carrier = cr.Name
	}
	if strings.TrimSpace(in.TrackingURL) == "" {
		in.TrackingURL = cr.TrackingURLFor(in.TrackingNo)
	}
	return nil
}

// resolveShipmentInputs applies resolveShipmentInput to a slice in place.
func (s *Service) resolveShipmentInputs(c *gin.Context, in []OrderShipmentInput) error {
	for i := range in {
		if err := s.resolveShipmentInput(c, &in[i]); err != nil {
			return err
		}
	}
	return nil
}
