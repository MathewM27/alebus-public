package httpapi

import (
	"context"
	"net/http"

	apihttp "github.com/MathewM27/busTrack-alebus/api/http"
)

// Scope is a thin alias over the canonical scope type defined in api/http.
// It lets the middleware layer reference scopes without circular imports.
type Scope = apihttp.Scope

const (
	ScopePublic = apihttp.ScopePublic
	ScopeOps    = apihttp.ScopeOps
	ScopeAdmin  = apihttp.ScopeAdmin
)

func ScopeFromContext(ctx context.Context) Scope { return apihttp.ScopeFromContext(ctx) }

func withScope(ctx context.Context, scope Scope) context.Context { return apihttp.WithScope(ctx, scope) }

func scopeAtLeast(got, required Scope) bool { return apihttp.ScopeAtLeast(got, required) }

// writeForbidden renders the canonical 403 JSON envelope.
func writeForbidden(w http.ResponseWriter, message string) {
	apihttp.WriteError(w, http.StatusForbidden, "forbidden", message, nil)
}
