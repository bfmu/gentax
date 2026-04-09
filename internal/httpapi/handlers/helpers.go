package handlers

import (
	"encoding/json"
	"net/http"
)

// writeJSON serialises v as JSON and writes it with the given HTTP status.
// The Content-Type header is always set to application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
