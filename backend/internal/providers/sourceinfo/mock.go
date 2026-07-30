package sourceinfo

import (
	"context"
	"hash/fnv"
	"time"
)

// Mock is a deterministic offline implementation used until the 1688
// open-platform API is available. Price/stock derive from a hash of the
// offer/SKU id plus the current hour, so refreshes show plausible drift
// without randomness in tests (a fixed Now yields fixed output).
type Mock struct {
	// Now allows tests to freeze time; defaults to time.Now.
	Now func() time.Time
}

// Platform implements Provider.
func (m *Mock) Platform() string { return "1688" }

func (m *Mock) now() time.Time {
	if m != nil && m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func seed(parts ...string) uint32 {
	h := fnv.New32a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
	}
	return h.Sum32()
}

// FetchOffer implements Provider with deterministic pseudo-quotes.
func (m *Mock) FetchOffer(_ context.Context, offerID string, externalSKUIDs []string) (*OfferQuote, error) {
	hour := m.now().UTC().Format("2006010215")
	if len(externalSKUIDs) == 0 {
		externalSKUIDs = []string{"default"}
	}
	out := &OfferQuote{OfferID: offerID}
	for _, sid := range externalSKUIDs {
		s := seed(offerID, sid)
		drift := seed(offerID, sid, hour)
		base := 5 + float64(s%9000)/100 // 5.00 ~ 94.99 CNY
		price := base * (1 + float64(drift%21)/100)
		stock := int(drift % 500)
		if drift%17 == 0 {
			stock = 0 // periodic simulated out-of-stock
		}
		out.SKUs = append(out.SKUs, SKUQuote{
			ExternalSKUID: sid,
			Price:         float64(int(price*100)) / 100,
			Currency:      "CNY",
			Stock:         stock,
		})
	}
	return out, nil
}
