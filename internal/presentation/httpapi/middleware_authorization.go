package httpapi

import "net/http"

// RequireScope enforces that the caller has at least the required scope.
func RequireScope(required Scope) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			got := ScopeFromContext(r.Context())
			if !scopeAtLeast(got, required) {
				writeForbidden(w, "insufficient scope")
				return
			}
			next(w, r)
		}
	}
}
