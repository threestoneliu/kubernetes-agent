package server

import (
	"net/http"
)

// healthHandler always returns 200 with the current provider
// statuses. We deliberately do not fail the health check on
// unreachable LLM providers — a degraded mode where the user can
// still inspect cluster state is more useful than a 503. The
// frontend reads the per-provider status to disable the chat box
// and surface a warning.
func healthHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var providers any = []any{}
		if d.LLM != nil {
			providers = d.LLM.Status()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"providers": providers,
		})
	}
}
