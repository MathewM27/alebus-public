package httpapi

import (
	"context"
	"strings"
)

type principalKey struct{}

// Principal is the caller identity used for authorization decisions.
//
// Today this is populated via headers by the presentation layer.
// In a future phase this should be derived from verified tokens/claims.
type Principal struct {
	UserID     string
	OperatorID string
}

func (p Principal) Normalized() Principal {
	p.UserID = strings.TrimSpace(p.UserID)
	p.OperatorID = strings.TrimSpace(p.OperatorID)
	return p
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p.Normalized())
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
