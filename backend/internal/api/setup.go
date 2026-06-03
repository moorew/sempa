package api

import (
	"errors"
	"net/http"

	"github.com/clevercode/sempa/internal/db"
	"github.com/google/uuid"
)

type setupHandler struct {
	configs *db.IntegrationConfigStore
}

const setupKey = "_setup_done"

func (h *setupHandler) status(w http.ResponseWriter, r *http.Request) {
	_, err := h.configs.Get(r.Context(), setupKey)
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]bool{"done": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]bool{"done": true})
}

func (h *setupHandler) complete(w http.ResponseWriter, r *http.Request) {
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), setupKey, "{}"); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]bool{"done": true})
}
