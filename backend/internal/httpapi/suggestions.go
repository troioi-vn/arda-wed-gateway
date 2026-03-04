package httpapi

import "net/http"

func (r *Router) handleSuggestionsLatest(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)
	status := r.suggestions.Status()
	payload := map[string]any{
		"in_progress": status.InProgress,
	}
	if status.LastError != "" {
		payload["last_error"] = status.LastError
		payload["last_error_at"] = status.LastErrorAt
	}

	suggestion, found := r.suggestions.Latest()
	if found {
		payload["commands"] = suggestion.Commands
		payload["reason"] = suggestion.Reason
		payload["expected_outcome"] = suggestion.ExpectedOutcome
		if suggestion.GeneratedAt != "" {
			payload["generated_at"] = suggestion.GeneratedAt
		}
	}

	writeSuccess(r.logger, w, http.StatusOK, requestID, payload)
}

func (r *Router) handleSuggestionsCancel(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)
	canceled := r.suggestions.CancelInFlight()
	writeSuccess(r.logger, w, http.StatusOK, requestID, map[string]any{
		"canceled": canceled,
	})
}
