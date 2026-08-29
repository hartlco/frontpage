package handlers

import (
	"log"
	"net/http"
)

type buildSyncer interface {
	Sync() error
}

type SyncHandler struct {
	syncer buildSyncer
}

func NewSyncHandler(syncer buildSyncer) *SyncHandler {
	return &SyncHandler{syncer: syncer}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result := "success"
	if err := h.syncer.Sync(); err != nil {
		log.Printf("manual git sync failed: %v", err)
		result = "error"
	}

	http.Redirect(w, r, "/?refresh="+result, http.StatusSeeOther)
}
