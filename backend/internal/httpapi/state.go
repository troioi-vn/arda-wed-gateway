package httpapi

import "net/http"

func (r *Router) handleStateSnapshot(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()

	snapshot, found, err := r.state.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to load state snapshot", nil)
		return
	}

	if !found {
		writeSuccess(w, http.StatusOK, requestID, map[string]any{})
		return
	}

	writeSuccess(w, http.StatusOK, requestID, snapshot)
}
