package mcpserver

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

type ctxKey struct{}

func withToken(ctx context.Context, tok *mcptoken.Token) context.Context {
	return context.WithValue(ctx, ctxKey{}, tok)
}

func tokenFromContext(ctx context.Context) (*mcptoken.Token, bool) {
	tok, ok := ctx.Value(ctxKey{}).(*mcptoken.Token)
	return tok, ok && tok != nil
}
