package httpapi

import "net/http"

func (r *Router) handleStateSnapshot(w http.ResponseWriter, req *http.Request) {
	requestID := r.requestID(req)

	snapshot, found, err := r.state.Snapshot()
	if err != nil {
		r.logger.Error("state snapshot load failed", "request_id", requestID, "error", err.Error())
		writeError(r.logger, w, http.StatusInternalServerError, requestID, "INTERNAL_ERROR", "failed to load state snapshot", nil)
		return
	}

	if !found {
		writeSuccess(r.logger, w, http.StatusOK, requestID, map[string]any{})
		return
	}

	writeSuccess(r.logger, w, http.StatusOK, requestID, snapshot)
}
