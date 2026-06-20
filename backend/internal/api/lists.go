package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/db"
)

// nowUTC matches the datetime('now') text format the DB uses elsewhere.
func nowUTC() string { return time.Now().UTC().Format("2006-01-02 15:04:05") }

type listHandler struct {
	store *db.ListStore
	hub   *EventHub
	sync  *db.SyncStore
}

func (h *listHandler) broadcast() {
	h.hub.Broadcast("list:change", map[string]string{"entity": "list"})
}

// ── Lists ────────────────────────────────────────────────────────────────────

func (h *listHandler) list(w http.ResponseWriter, r *http.Request) {
	lists, err := h.store.List(r.Context(),
		r.URL.Query().Get("archived") == "1",
		r.URL.Query().Get("task_id"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load lists")
		return
	}
	respond(w, http.StatusOK, lists)
}

func (h *listHandler) get(w http.ResponseWriter, r *http.Request) {
	l, err := h.store.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	respond(w, http.StatusOK, l)
}

func (h *listHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string  `json:"name"`
		TaskID *string `json:"task_id"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := h.store.Create(r.Context(), db.CreateListParams{
		ID: uuid.New().String(), Name: req.Name, TaskID: req.TaskID,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create list")
		return
	}
	h.broadcast()
	respond(w, http.StatusCreated, l)
}

func (h *listHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	l, err := h.store.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	var req struct {
		Name              *string `json:"name"`
		TaskID            *string `json:"task_id"` // "" unlinks; an id links; omitted = unchanged
		Position          *float64 `json:"position"`
		Archived          *bool    `json:"archived"`
		ArchiveOnComplete *bool    `json:"archive_on_complete"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil {
		l.Name = *req.Name
	}
	if req.TaskID != nil {
		if *req.TaskID == "" {
			l.TaskID = nil
		} else {
			l.TaskID = req.TaskID
		}
	}
	if req.Position != nil {
		l.Position = *req.Position
	}
	if req.Archived != nil {
		if *req.Archived {
			now := nowUTC()
			l.ArchivedAt = &now
		} else {
			l.ArchivedAt = nil
		}
	}
	if req.ArchiveOnComplete != nil {
		l.ArchiveOnComplete = *req.ArchiveOnComplete
	}
	updated, err := h.store.Update(r.Context(), l)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update list")
		return
	}
	h.broadcast()
	respond(w, http.StatusOK, updated)
}

func (h *listHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Cascade removes items in the DB, but clients need tombstones for each.
	items, _ := h.store.Items(r.Context(), id)
	if err := h.store.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete list")
		return
	}
	_ = h.sync.RecordTombstone(r.Context(), "list", id)
	for _, it := range items {
		_ = h.sync.RecordTombstone(r.Context(), "list_item", it.ID)
	}
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Items ────────────────────────────────────────────────────────────────────

func (h *listHandler) items(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Items(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load items")
		return
	}
	respond(w, http.StatusOK, items)
}

func (h *listHandler) createItem(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	var req struct {
		Text string `json:"text"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	it, err := h.store.CreateItem(r.Context(), db.CreateItemParams{
		ID: uuid.New().String(), ListID: listID, Text: req.Text,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add item")
		return
	}
	h.broadcast()
	respond(w, http.StatusCreated, it)
}

func (h *listHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	it, err := h.store.GetItem(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusNotFound, "item not found")
		return
	}
	var req struct {
		Text     *string  `json:"text"`
		Done     *bool    `json:"done"`
		Position *float64 `json:"position"`
		Category *string  `json:"category"` // "" clears
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Text != nil {
		it.Text = *req.Text
	}
	if req.Done != nil {
		it.Done = *req.Done
	}
	if req.Position != nil {
		it.Position = *req.Position
	}
	if req.Category != nil {
		if *req.Category == "" {
			it.Category = nil
		} else {
			it.Category = req.Category
		}
	}
	updated, err := h.store.UpdateItem(r.Context(), it)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update item")
		return
	}
	h.broadcast()
	respond(w, http.StatusOK, updated)
}

func (h *listHandler) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteItem(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete item")
		return
	}
	_ = h.sync.RecordTombstone(r.Context(), "list_item", id)
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *listHandler) reorderItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.Reorder(r.Context(), req.IDs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to reorder")
		return
	}
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
