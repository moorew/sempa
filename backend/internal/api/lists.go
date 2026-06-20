package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
		r.URL.Query().Get("task_id"),
		ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load lists")
		return
	}
	respond(w, http.StatusOK, lists)
}

func (h *listHandler) get(w http.ResponseWriter, r *http.Request) {
	l, err := h.store.Get(r.Context(), chi.URLParam(r, "id"), ownerID(r))
	if err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	respond(w, http.StatusOK, l)
}

func (h *listHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		TaskID *string `json:"task_id"`
		Shared bool    `json:"shared"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	l, err := h.store.Create(r.Context(), db.CreateListParams{
		ID: clientOrNewID(req.ID), Name: req.Name, TaskID: req.TaskID,
		OwnerID: ownerID(r), Shared: req.Shared,
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
	owner := ownerID(r)
	l, err := h.store.Get(r.Context(), id, owner)
	if err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	wasShared := l.Shared
	var req struct {
		Name              *string  `json:"name"`
		TaskID            *string  `json:"task_id"` // "" unlinks; an id links; omitted = unchanged
		Position          *float64 `json:"position"`
		Archived          *bool    `json:"archived"`
		ArchiveOnComplete *bool    `json:"archive_on_complete"`
		Shared            *bool    `json:"shared"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Shared != nil {
		l.Shared = *req.Shared
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
	// Keep items in lockstep with the list's share state; on un-share, revoke the
	// list + each item from peers (they drop their stale copies; owner keeps them).
	if req.Shared != nil && updated.Shared != wasShared {
		_ = h.store.SetItemsShared(r.Context(), updated.ID, updated.Shared)
		if !updated.Shared {
			items, _ := h.store.Items(r.Context(), updated.ID, owner)
			_ = h.sync.RecordRevocation(r.Context(), "list", updated.ID, owner)
			for _, it := range items {
				_ = h.sync.RecordRevocation(r.Context(), "list_item", it.ID, owner)
			}
		}
	}
	h.broadcast()
	respond(w, http.StatusOK, updated)
}

func (h *listHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	owner := ownerID(r)
	// Scoped Get gates the delete and tells us how to route the tombstones.
	l, err := h.store.Get(r.Context(), id, owner)
	if err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	// Cascade removes items in the DB, but clients need tombstones for each.
	items, _ := h.store.Items(r.Context(), id, owner)
	if err := h.store.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete list")
		return
	}
	_ = h.sync.RecordTombstone(r.Context(), "list", id, l.OwnerID, l.Shared)
	for _, it := range items {
		_ = h.sync.RecordTombstone(r.Context(), "list_item", it.ID, it.OwnerID, it.Shared)
	}
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Items ────────────────────────────────────────────────────────────────────

func (h *listHandler) items(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Items(r.Context(), chi.URLParam(r, "id"), ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load items")
		return
	}
	respond(w, http.StatusOK, items)
}

func (h *listHandler) createItem(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	// Only add items to a list you can see; the store inherits the item's
	// owner/shared from that list.
	if _, err := h.store.Get(r.Context(), listID, ownerID(r)); err != nil {
		respondError(w, http.StatusNotFound, "list not found")
		return
	}
	var req struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	it, err := h.store.CreateItem(r.Context(), db.CreateItemParams{
		ID: clientOrNewID(req.ID), ListID: listID, Text: req.Text,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add item")
		return
	}
	h.broadcast()
	respond(w, http.StatusCreated, it)
}

func (h *listHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	it, err := h.store.GetItem(r.Context(), chi.URLParam(r, "id"), ownerID(r))
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
	// Scoped Get gates the delete and routes the tombstone.
	it, err := h.store.GetItem(r.Context(), id, ownerID(r))
	if err != nil {
		respondError(w, http.StatusNotFound, "item not found")
		return
	}
	if err := h.store.DeleteItem(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete item")
		return
	}
	_ = h.sync.RecordTombstone(r.Context(), "list_item", id, it.OwnerID, it.Shared)
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
