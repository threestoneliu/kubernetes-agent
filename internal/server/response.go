package server

import (
	"encoding/json"
	"net/http"
)

// writeJSON serialises body as JSON and writes it with the given
// status. It always sets Content-Type to application/json. Errors
// from the encoder are not surfaced (we can't write headers after
// the body has started).
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errorResponse is the standard wire shape for 4xx / 5xx returns.
// The retryable flag tells the frontend whether to show a "retry"
// button (transient errors) vs a hard "report" button.
type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// writeError emits the standard error envelope. Convenience wrapper
// over writeJSON so handlers don't repeat the literal struct type.
func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(w, status, errorResponse{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	})
}

// decodeJSON binds the request body to dst. Returns false if the
// decode failed; the helper writes the 400 response so handlers can
// just `if !decodeJSON(w, r, &req) { return }`.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error(), false)
		return false
	}
	return true
}
