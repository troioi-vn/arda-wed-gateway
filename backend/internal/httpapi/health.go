package httpapi

import (
	"net/http"
)

type healthPayload struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	requestID := r.nextRequestID()
	writeSuccess(w, http.StatusOK, requestID, healthPayload{
		Status:  "ok",
		Version: r.cfg.BuildVersion,
		Commit:  r.cfg.BuildCommit,
	})
}
