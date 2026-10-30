package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/clevercode/sempa/internal/db"
)

type projectHandler struct {
	store  *db.ProjectStore
	hub    *EventHub
	sync   *db.SyncStore
	attach *attachmentHandler // for cascading attachment cleanup on delete
}

func (h *projectHandler) broadcast() {
	h.hub.Broadcast("project:change", map[string]string{"entity": "project"})
}

// ── Projects ─────────────────────────────────────────────────────────────────

func (h *projectHandler) list(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.List(r.Context(), ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load projects")
		return
	}
	respond(w, http.StatusOK, projects)
}

func (h *projectHandler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.Get(r.Context(), chi.URLParam(r, "id"), ownerID(r))
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}
	respond(w, http.StatusOK, p)
}

func (h *projectHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description *string `json:"description"`
		StatusID    string  `json:"status_id"`
		Color       string  `json:"color"`
		StartDate   *string `json:"start_date"`
		TargetDate  *string `json:"target_date"`
		Shared      bool    `json:"shared"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	owner := ownerID(r)
	// Idempotent replay of an offline create (same reasoning as tasks).
	if req.ID != "" {
		if existing, err := h.store.Get(r.Context(), req.ID, owner); err == nil {
			respond(w, http.StatusOK, existing)
			return
		}
	}
	p, err := h.store.Create(r.Context(), db.CreateProjectParams{
		ID: clientOrNewID(req.ID), Title: req.Title, Description: req.Description,
		StatusID: req.StatusID, Color: req.Color,
		StartDate: emptyToNil(req.StartDate), TargetDate: emptyToNil(req.TargetDate),
		OwnerID: owner, Shared: req.Shared,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create project")
		return
	}
	if h.sync != nil {
		_ = h.sync.ClearTombstone(r.Context(), "project", p.ID)
	}
	h.broadcast()
	respond(w, http.StatusCreated, p)
}

func (h *projectHandler) update(w http.ResponseWriter, r *http.Request) {
	owner := ownerID(r)
	p, err := h.store.Get(r.Context(), chi.URLParam(r, "id"), owner)
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}
	wasShared := p.Shared
	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		StatusID    *string  `json:"status_id"`
		Color       *string  `json:"color"`
		StartDate   *string  `json:"start_date"`  // "" clears
		TargetDate  *string  `json:"target_date"` // "" clears
		Position    *float64 `json:"position"`
		Shared      *bool    `json:"shared"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title != nil {
		p.Title = *req.Title
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.StatusID != nil && *req.StatusID != "" {
		p.StatusID = *req.StatusID
	}
	if req.Color != nil && *req.Color != "" {
		p.Color = *req.Color
	}
	if req.StartDate != nil {
		p.StartDate = emptyToNil(req.StartDate)
	}
	if req.TargetDate != nil {
		p.TargetDate = emptyToNil(req.TargetDate)
	}
	if req.Position != nil {
		p.Position = *req.Position
	}
	if req.Shared != nil {
		p.Shared = *req.Shared
	}
	updated, err := h.store.Update(r.Context(), p)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update project")
		return
	}
	// Un-share: peers drop their copy (the owner keeps it). Linked tasks etc. keep
	// their own share state — a private task can sit in a shared project.
	if req.Shared != nil && wasShared && !updated.Shared && h.sync != nil {
		_ = h.sync.RecordRevocation(r.Context(), "project", updated.ID, owner)
	}
	h.broadcast()
	respond(w, http.StatusOK, updated)
}

func (h *projectHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.store.Get(r.Context(), id, ownerID(r))
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}
	links, err := h.store.Delete(r.Context(), id)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	if h.attach != nil {
		h.attach.removeForOwner(r, "project", id)
	}
	if h.sync != nil {
		_ = h.sync.RecordTombstone(r.Context(), "project", id, p.OwnerID, p.Shared)
	}
	h.broadcast()
	// Unlinked rows changed too; nudge the views that show them.
	if links.Tasks > 0 {
		h.hub.Broadcast("task:change", map[string]string{"entity": "task"})
	}
	if links.Objectives > 0 {
		h.hub.Broadcast("objective:change", map[string]string{"entity": "objective"})
	}
	if links.Lists > 0 {
		h.hub.Broadcast("list:change", map[string]string{"entity": "list"})
	}
	respond(w, http.StatusOK, map[string]any{"status": "deleted", "unlinked": links})
}

// ── Project statuses ─────────────────────────────────────────────────────────

func (h *projectHandler) statuses(w http.ResponseWriter, r *http.Request) {
	sts, err := h.store.Statuses(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load project statuses")
		return
	}
	respond(w, http.StatusOK, sts)
}

func (h *projectHandler) createStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Color    string `json:"color"`
		Category string `json:"category"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Category == "" {
		req.Category = "active"
	}
	if !db.ValidProjectCategory(req.Category) {
		respondError(w, http.StatusBadRequest, "invalid category")
		return
	}
	if req.Color == "" {
		req.Color = "#8a8178"
	}
	// Idempotent replay of an offline create.
	if req.ID != "" {
		if existing, err := h.store.GetStatus(r.Context(), req.ID); err == nil {
			respond(w, http.StatusOK, existing)
			return
		}
	}
	st, err := h.store.CreateStatus(r.Context(), db.ProjectStatus{
		ID: clientOrNewID(req.ID), Name: req.Name, Color: req.Color, Category: req.Category,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create status")
		return
	}
	if h.sync != nil {
		_ = h.sync.ClearTombstone(r.Context(), "project_status", st.ID)
	}
	h.broadcast()
	respond(w, http.StatusCreated, st)
}

func (h *projectHandler) updateStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.GetStatus(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusNotFound, "status not found")
		return
	}
	var req struct {
		Name     *string  `json:"name"`
		Color    *string  `json:"color"`
		Category *string  `json:"category"`
		Position *float64 `json:"position"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		st.Name = strings.TrimSpace(*req.Name)
	}
	if req.Color != nil && *req.Color != "" {
		st.Color = *req.Color
	}
	if req.Category != nil {
		if !db.ValidProjectCategory(*req.Category) {
			respondError(w, http.StatusBadRequest, "invalid category")
			return
		}
		st.Category = *req.Category
	}
	if req.Position != nil {
		st.Position = *req.Position
	}
	updated, err := h.store.UpdateStatus(r.Context(), st)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	h.broadcast()
	respond(w, http.StatusOK, updated)
}

func (h *projectHandler) deleteStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	replacement, err := h.store.DeleteStatus(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrNotFound):
		respondError(w, http.StatusNotFound, "status not found")
		return
	case errors.Is(err, db.ErrLastStatus):
		respondError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		respondError(w, http.StatusInternalServerError, "failed to delete status")
		return
	}
	// Statuses are global, like tags: the tombstone reaches everyone.
	if h.sync != nil {
		_ = h.sync.RecordTombstone(r.Context(), "project_status", id, "", false)
	}
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "deleted", "replacement": replacement})
}

func (h *projectHandler) reorderStatuses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.ReorderStatuses(r.Context(), req.IDs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to reorder")
		return
	}
	h.broadcast()
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
