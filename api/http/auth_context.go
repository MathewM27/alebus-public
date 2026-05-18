package httpapi

import "context"

type scopeKey struct{}

type Scope string

const (
	ScopePublic Scope = "public"
	ScopeOps    Scope = "ops"
	ScopeAdmin  Scope = "admin"
)

func scopeRank(s Scope) int {
	switch s {
	case ScopeAdmin:
		return 3
	case ScopeOps:
		return 2
	default:
		return 1
	}
}

func ScopeAtLeast(got Scope, required Scope) bool {
	return scopeRank(got) >= scopeRank(required)
}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func ScopeFromContext(ctx context.Context) Scope {
	if v, ok := ctx.Value(scopeKey{}).(Scope); ok {
		return v
	}
	return ""
}
