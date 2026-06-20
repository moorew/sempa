package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/clevercode/sempa/internal/db"
)

// Pairing lets a device (the Sempa Dock) join the account via a short code
// approved from an already-signed-in app — no password ever lands on the device.
// Approval mints a normal session token (so existing auth validates it) with a
// bounded TTL; revoking deletes that session.
type pairingHandler struct {
	store    *db.PairingStore
	sessions *sessionStore
}

// pairingTTL: how long an unapproved code is valid. deviceTokenTTL: the minted
// device session lifetime — kept to ~the current week (scoped lifetime), so a
// lost device ages out even if never revoked.
const (
	pairingTTL     = 10 * time.Minute
	deviceTokenTTL = 8 * 24 * time.Hour
)

type startPairingRequest struct {
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
}

// POST /api/v1/devices/pair/start (public) — the device requests a pairing code.
func (h *pairingHandler) start(w http.ResponseWriter, r *http.Request) {
	var req startPairingRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Platform == "" {
		req.Platform = "dock"
	}
	d, err := h.store.CreatePending(req.DeviceName, req.Platform, pairingTTL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start pairing")
		return
	}
	respond(w, http.StatusCreated, map[string]any{
		"code":       d.Code,
		"expires_in": int(pairingTTL.Seconds()),
	})
}

// GET /api/v1/devices/pair/status?code=… (public) — the device polls until
// approved, then receives its token exactly once.
func (h *pairingHandler) status(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, http.StatusBadRequest, "code required")
		return
	}
	d, err := h.store.GetByCode(code)
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"status": "unknown"})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	switch d.Status {
	case "approved":
		// Hand the token over once; later polls get status only.
		token, err := h.store.ClaimToken(code)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "claim failed")
			return
		}
		resp := map[string]any{"status": "approved"}
		if token != "" {
			resp["token"] = token
		}
		respond(w, http.StatusOK, resp)
	case "revoked":
		respond(w, http.StatusOK, map[string]any{"status": "revoked"})
	default: // pending
		if d.ExpiresAt < time.Now().UTC().Format(time.RFC3339) {
			respond(w, http.StatusOK, map[string]any{"status": "expired"})
			return
		}
		respond(w, http.StatusOK, map[string]any{"status": "pending"})
	}
}

type approvePairingRequest struct {
	Code string `json:"code"`
}

// POST /api/v1/devices/pair/approve (authed) — the signed-in app approves a code.
func (h *pairingHandler) approve(w http.ResponseWriter, r *http.Request) {
	var req approvePairingRequest
	if err := decode(r, &req); err != nil || req.Code == "" {
		respondError(w, http.StatusBadRequest, "code required")
		return
	}
	// Mint a DEVICE-SCOPED session (restricted allowlist — see deviceAllowed),
	// then attach it. If the code is bad/expired the Approve fails and the
	// freshly-minted session is dropped immediately.
	token := h.sessions.createScoped(deviceTokenTTL, "dock", "", "device")
	d, err := h.store.Approve(req.Code, token)
	if errors.Is(err, db.ErrNotFound) {
		h.sessions.delete(token)
		respondError(w, http.StatusNotFound, "code not found or expired")
		return
	}
	if err != nil {
		h.sessions.delete(token)
		respondError(w, http.StatusInternalServerError, "approve failed")
		return
	}
	respond(w, http.StatusOK, map[string]any{"device_name": d.DeviceName, "platform": d.Platform})
}

// GET /api/v1/devices (authed) — list paired devices for the account.
func (h *pairingHandler) list(w http.ResponseWriter, r *http.Request) {
	devices, err := h.store.List()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "list failed")
		return
	}
	respond(w, http.StatusOK, devices)
}

// DELETE /api/v1/devices/{id} (authed) — revoke a device + its session.
func (h *pairingHandler) revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	token, err := h.store.Revoke(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "revoke failed")
		return
	}
	if token != "" {
		h.sessions.delete(token)
	}
	w.WriteHeader(http.StatusNoContent)
}
