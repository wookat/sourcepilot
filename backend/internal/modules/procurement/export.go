package procurement

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// csvHeader is the manual purchase list header (1688 link included so the
// operator can order by hand).
var csvHeader = []string{
	"采购单号", "供应商", "1688商品链接", "1688 offerId", "货源SKU", "商品标题", "SKU名称", "数量", "参考单价(CNY)", "小计(CNY)", "采购单状态",
}

func offerLink(sourceURL, offerID string) string {
	if u := strings.TrimSpace(sourceURL); u != "" {
		return u
	}
	if offerID != "" {
		return fmt.Sprintf("https://detail.1688.com/offer/%s.html", offerID)
	}
	return ""
}

func writePORows(w *csv.Writer, po *PurchaseOrder) error {
	for _, it := range po.Items {
		price := 0.0
		if it.ActualPrice != nil {
			price = *it.ActualPrice
		} else if it.ExpectedPrice != nil {
			price = *it.ExpectedPrice
		}
		row := []string{
			po.ID.String(),
			po.SupplierName,
			offerLink(it.SourceURL, it.ExternalOfferID),
			it.ExternalOfferID,
			it.ExternalSKUID,
			it.ProductTitle,
			it.SKUName,
			fmt.Sprintf("%d", it.Quantity),
			fmt.Sprintf("%.2f", price),
			fmt.Sprintf("%.2f", price*float64(it.Quantity)),
			po.Status,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// ExportCSV renders the manual purchase list for one purchase order.
func (s *Service) ExportCSV(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	po, err := s.Detail(ctx, id)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return nil, "", err
	}
	if err := writePORows(w, po); err != nil {
		return nil, "", err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("purchase-list-%s.csv", po.ID.String()[:8])
	return buf.Bytes(), name, nil
}

// ExportBatchCSV renders one merged manual purchase list covering several
// purchase orders (one row per item, 采购单号 column distinguishes orders).
func (s *Service) ExportBatchCSV(ctx context.Context, ids []uuid.UUID) ([]byte, string, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return nil, "", err
	}
	for _, id := range ids {
		po, err := s.Detail(ctx, id)
		if err != nil {
			return nil, "", err
		}
		if err := writePORows(w, po); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("purchase-lists-%d.csv", len(ids))
	return buf.Bytes(), name, nil
}
