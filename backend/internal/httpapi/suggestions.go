package httpapi

import "net/http"

func (r *Router) handleSuggestionsLatest(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()

	suggestion, found := r.suggestions.Latest()
	if !found {
		writeSuccess(w, http.StatusOK, requestID, map[string]any{})
		return
	}

	writeSuccess(w, http.StatusOK, requestID, suggestion)
}
