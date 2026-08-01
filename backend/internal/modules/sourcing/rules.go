package sourcing

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// RuleConfig holds supplier switch thresholds (settings group "sourcing").
type RuleConfig struct {
	// PriceIncreaseThresholdPercent marks a source price_alert when the
	// latest price rises above this percent versus the previous snapshot.
	PriceIncreaseThresholdPercent float64
	// AutoSwitchOnOutOfStock switches primary to the best backup source
	// automatically when the primary goes out of stock (locked sources skip).
	AutoSwitchOnOutOfStock bool
	// AutoSwitchOnPriceAlert switches automatically on price alerts;
	// default false → only a suggested switch event is produced.
	AutoSwitchOnPriceAlert bool
}

func defaultRuleConfig() RuleConfig {
	return RuleConfig{PriceIncreaseThresholdPercent: 10, AutoSwitchOnOutOfStock: true, AutoSwitchOnPriceAlert: false}
}

func (s *Service) ruleConfig(ctx context.Context) (RuleConfig, error) {
	cfg := defaultRuleConfig()
	if s.Settings == nil {
		return cfg, nil
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "sourcing")
	if err != nil {
		return cfg, nil // missing group falls back to defaults
	}
	if v := strings.TrimSpace(plain["price_increase_threshold_percent"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.PriceIncreaseThresholdPercent = f
		}
	}
	if v := strings.TrimSpace(plain["auto_switch_on_out_of_stock"]); v != "" {
		cfg.AutoSwitchOnOutOfStock = v == "true" || v == "1"
	}
	if v := strings.TrimSpace(plain["auto_switch_on_price_alert"]); v != "" {
		cfg.AutoSwitchOnPriceAlert = v == "true" || v == "1"
	}
	return cfg, nil
}

// pickBackup returns the best available backup source: active status,
// not disabled, lowest priority number, excluding the current primary.
func pickBackup(sources []ProductSource, exclude uuid.UUID) *ProductSource {
	var best *ProductSource
	for i := range sources {
		src := &sources[i]
		if src.ID == exclude || src.Status != SourceStatusActive {
			continue
		}
		if best == nil || src.Priority < best.Priority {
			best = src
		}
	}
	return best
}

// applySwitchRules evaluates the primary source after a refresh and either
// switches automatically (out-of-stock) or records a suggested switch
// (price alert), following module_design.md §2.3.
func (s *Service) applySwitchRules(ctx context.Context, productID uuid.UUID, sources []ProductSource, cfg RuleConfig) (*ProductSource, []RefreshAlert, error) {
	var primary *ProductSource
	for i := range sources {
		if sources[i].IsPrimary {
			primary = &sources[i]
			break
		}
	}
	if primary == nil {
		return nil, nil, nil
	}
	var alerts []RefreshAlert
	needSwitch := false
	reason := ""
	mode := SwitchModeAuto
	switch primary.Status {
	case SourceStatusOutOfStock:
		reason = SwitchReasonOutOfStock
		if primary.Locked {
			alerts = append(alerts, RefreshAlert{Code: AlertPrimaryLocked, SourceID: &primary.ID, SupplierName: supplierName(primary), Reason: reason})
		} else if cfg.AutoSwitchOnOutOfStock {
			needSwitch = true
		} else {
			mode = SwitchModeSuggested
		}
	case SourceStatusPriceAlert:
		reason = SwitchReasonPriceIncrease
		if primary.Locked {
			alerts = append(alerts, RefreshAlert{Code: AlertPrimaryLocked, SourceID: &primary.ID, SupplierName: supplierName(primary), Reason: reason})
		} else if cfg.AutoSwitchOnPriceAlert {
			needSwitch = true
		} else {
			mode = SwitchModeSuggested
		}
	default:
		return nil, alerts, nil
	}
	backup := pickBackup(sources, primary.ID)
	if backup == nil {
		alerts = append(alerts, RefreshAlert{Code: AlertNoBackup, SourceID: &primary.ID, SupplierName: supplierName(primary), Reason: reason})
		return nil, alerts, nil
	}
	detail, _ := json.Marshal(map[string]any{
		"threshold_percent": cfg.PriceIncreaseThresholdPercent,
		"primary_status":    primary.Status,
	})
	if !needSwitch {
		if reason != "" && !primary.Locked {
			var dup int64
			if err := s.DB.WithContext(ctx).Model(&SourceSwitchEvent{}).
				Where("product_id = ? AND from_source_id = ? AND to_source_id = ? AND reason = ? AND mode = ? AND status = ?",
					productID, primary.ID, backup.ID, reason, SwitchModeSuggested, SuggestionOpen).
				Count(&dup).Error; err != nil {
				return nil, alerts, err
			}
			if dup == 0 {
				ev := SourceSwitchEvent{
					ProductID:    productID,
					FromSourceID: &primary.ID,
					ToSourceID:   backup.ID,
					Reason:       reason,
					Detail:       datatypes.JSON(detail),
					Mode:         SwitchModeSuggested,
					Status:       SuggestionOpen,
				}
				if err := s.DB.WithContext(ctx).Create(&ev).Error; err != nil {
					return nil, alerts, err
				}
			}
			alerts = append(alerts, RefreshAlert{Code: AlertSwitchSuggested, SourceID: &backup.ID, SupplierName: supplierName(backup), Reason: reason})
		}
		_ = mode
		return nil, alerts, nil
	}
	if err := s.DB.WithContext(ctx).Model(&ProductSource{}).
		Where("product_id = ? AND is_primary = TRUE", productID).
		Update("is_primary", false).Error; err != nil {
		return nil, alerts, err
	}
	if err := s.DB.WithContext(ctx).Model(&ProductSource{}).
		Where("id = ?", backup.ID).Update("is_primary", true).Error; err != nil {
		return nil, alerts, err
	}
	ev := SourceSwitchEvent{
		ProductID:    productID,
		FromSourceID: &primary.ID,
		ToSourceID:   backup.ID,
		Reason:       reason,
		Detail:       datatypes.JSON(detail),
		Mode:         SwitchModeAuto,
	}
	if err := s.DB.WithContext(ctx).Create(&ev).Error; err != nil {
		return nil, alerts, err
	}
	backup.IsPrimary = true
	primary.IsPrimary = false
	alerts = append(alerts, RefreshAlert{Code: AlertAutoSwitched, SourceID: &backup.ID, SupplierName: supplierName(backup), Reason: reason})
	return backup, alerts, nil
}
