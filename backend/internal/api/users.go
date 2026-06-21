package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/clevercode/sempa/internal/db"
)

const minPasswordLen = 8

// currentUser resolves the logged-in user row from the request's session
// (by id, falling back to email for pre-migration sessions).
func (h *authHandler) currentUser(r *http.Request) (db.User, bool) {
	e, ok := currentSession(r)
	if !ok {
		return db.User{}, false
	}
	return h.resolveUser(r.Context(), e)
}

func (h *authHandler) isAdmin(r *http.Request) bool {
	u, ok := h.currentUser(r)
	return ok && u.IsAdmin
}

// listUsers — admin only.
func (h *authHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin only")
		return
	}
	users, err := h.users.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respond(w, http.StatusOK, users)
}

// createUser — admin only. Creates a password account.
func (h *authHandler) createUser(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin only")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		respondError(w, http.StatusUnprocessableEntity, "email is required")
		return
	}
	// Passwordless = a Google-only invite: the account exists immediately and the
	// person signs in with Google (the invited DB row passes the auth gate). A
	// password is only required (and only stored) for credential accounts.
	var hashPtr *string
	if req.Password != "" {
		if len(req.Password) < minPasswordLen {
			respondError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
			return
		}
		hash, err := db.HashPassword(req.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		hashPtr = &hash
	}
	user, err := h.users.Create(r.Context(), db.CreateUserParams{
		Email: req.Email, Name: req.Name, PasswordHash: hashPtr, IsAdmin: req.IsAdmin,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			respondError(w, http.StatusConflict, "a user with that email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	respond(w, http.StatusCreated, user)
}

// deleteUser — admin only; can't delete yourself.
func (h *authHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentUser(r)
	if !ok || !me.IsAdmin {
		respondError(w, http.StatusForbidden, "admin only")
		return
	}
	id := chi.URLParam(r, "id")
	if id == me.ID {
		respondError(w, http.StatusBadRequest, "you can't delete your own account")
		return
	}
	if err := h.users.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// adminSetPassword — admin resets another user's password.
func (h *authHandler) adminSetPassword(w http.ResponseWriter, r *http.Request) {
	if !h.isAdmin(r) {
		respondError(w, http.StatusForbidden, "admin only")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Password) < minPasswordLen {
		respondError(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}
	id := chi.URLParam(r, "id")
	if _, err := h.users.GetByID(r.Context(), id); errors.Is(err, db.ErrUserNotFound) {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if err := h.users.SetPassword(r.Context(), id, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// changeOwnPassword — any logged-in user changes their own password.
func (h *authHandler) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	me, ok := h.currentUser(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.NewPassword) < minPasswordLen {
		respondError(w, http.StatusUnprocessableEntity, "new password must be at least 8 characters")
		return
	}
	// If they already have a password, the current one must match.
	if me.HasPassword && !db.CheckPassword(*me.PasswordHash, req.CurrentPassword) {
		respondError(w, http.StatusForbidden, "current password is incorrect")
		return
	}
	hash, err := db.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if err := h.users.SetPassword(r.Context(), me.ID, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}
