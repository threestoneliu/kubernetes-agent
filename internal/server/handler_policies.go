package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/threestoneliu/kubernetes-agent/internal/policy"
	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// policyView is the public projection of a stored policy. We expose
// the raw YAML so the editor can round-trip the user changes
// without a separate "fetch rule definition" call.
type policyView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	YAML      string `json:"yaml"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func toPolicyView(p store.Policy) policyView {
	return policyView{
		ID:        p.ID,
		Name:      p.Name,
		YAML:      p.YAML,
		Enabled:   p.Enabled,
		CreatedAt: p.CreatedAt.Unix(),
		UpdatedAt: p.UpdatedAt.Unix(),
	}
}

func listPoliciesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Return all rules (enabled + disabled) so the editor can
		// re-enable a rule the user previously turned off.
		all, err := d.DB.ListAllPolicies(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		out := make([]policyView, 0, len(all))
		for _, p := range all {
			out = append(out, toPolicyView(p))
		}
		writeJSON(w, http.StatusOK, map[string]any{"policies": out})
	}
}

type updatePolicyReq struct {
	YAML string `json:"yaml"`
}

func updatePolicyHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req updatePolicyReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.YAML == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "yaml is required", false)
			return
		}
		// Parse into policy.Rule so a syntax / shape error fails
		// fast (400) instead of silently corrupting the policy
		// store.
		var rule policy.Rule
		if err := yaml.Unmarshal([]byte(req.YAML), &rule); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_yaml", err.Error(), false)
			return
		}
		if rule.Name == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "rule.name is required", false)
			return
		}
		if rule.Effect == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "rule.effect is required", false)
			return
		}
		// Preserve the existing enabled flag — the toggle endpoint
		// owns that, the YAML update only owns the rule body.
		existing, err := getPolicyByID(r, d, id)
		if err != nil {
			policyLookupError(w, err)
			return
		}
		row := store.Policy{
			ID:      id,
			Name:    rule.Name,
			YAML:    req.YAML,
			Enabled: existing.Enabled,
		}
		if err := d.DB.UpsertPolicy(r.Context(), row); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		writeAudit(r, d, "policy_update", id)
		writeJSON(w, http.StatusOK, toPolicyView(row))
	}
}

type togglePolicyReq struct {
	Enabled bool `json:"enabled"`
}

func togglePolicyHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req togglePolicyReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := d.DB.SetEnabled(r.Context(), id, req.Enabled); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "policy not found", false)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		writeAudit(r, d, "policy_toggle", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// getPolicyByID fetches a single policy by id. Returns
// store.ErrNotFound if it doesn't exist. Implemented locally to
// avoid adding a method to the store package just for the HTTP
// layer.
func getPolicyByID(r *http.Request, d Deps, id string) (store.Policy, error) {
	rows, err := d.DB.ListAllPolicies(r.Context())
	if err != nil {
		return store.Policy{}, err
	}
	for _, p := range rows {
		if p.ID == id {
			return p, nil
		}
	}
	return store.Policy{}, store.ErrNotFound
}

func policyLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "policy not found", false)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
}

// writeAudit is a thin shim around DB.AppendAudit used by policy
// mutators. It logs actor="user" — the MVP has no auth, so every
// change is attributed to the same opaque identity.
func writeAudit(r *http.Request, d Deps, action, target string) {
	if d.DB == nil {
		return
	}
	actor := "user"
	_, _ = d.DB.AppendAudit(r.Context(), store.AuditEntry{
		Action: action,
		Target: &target,
		Status: "ok",
		Message: func() *string {
			s := "actor=" + actor
			return &s
		}(),
	})
}
