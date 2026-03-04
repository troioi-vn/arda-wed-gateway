package httpapi

import "net/http"

func (r *Router) handleSuggestionsLatest(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)

	suggestion, found := r.suggestions.Latest()
	if !found {
		writeSuccess(r.logger, w, http.StatusOK, requestID, map[string]any{})
		return
	}

	writeSuccess(r.logger, w, http.StatusOK, requestID, suggestion)
}
