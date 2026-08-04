package goofish

import platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"

// RegisterProvider registers the Goofish (闲鱼) beta publish provider.
func RegisterProvider() {
	platformp.Register(NewProvider())
}
