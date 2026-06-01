package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/clevercode/aura/internal/config"
	"github.com/clevercode/aura/internal/db"
	"github.com/clevercode/aura/internal/integrations/gmail"
	"github.com/clevercode/aura/internal/integrations/jira"
)

type integrationHandler struct {
	configs *db.IntegrationConfigStore
	tasks   *db.TaskStore
	cfg     config.Config
}

// ── Jira ─────────────────────────────────────────────────────────────────────

func (h *integrationHandler) jiraGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var raw jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &raw); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	raw.APIToken = "" // never send the token back to the client

	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        cfg.Enabled,
		"last_synced_at": cfg.LastSyncedAt,
		"config":         raw,
	})
}

func (h *integrationHandler) jiraPut(w http.ResponseWriter, r *http.Request) {
	var body jira.Config
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Host == "" || body.Email == "" || body.APIToken == "" {
		respondError(w, http.StatusBadRequest, "host, email, and api_token are required")
		return
	}

	// If updating and existing record has the token, preserve existing token if
	// the client sent an empty string (redacted round-trip).
	if body.APIToken == "" {
		existing, err := h.configs.Get(r.Context(), "jira")
		if err == nil {
			var prev jira.Config
			if json.Unmarshal([]byte(existing.Config), &prev) == nil {
				body.APIToken = prev.APIToken
			}
		}
	}

	configJSON, err := json.Marshal(body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	cfg, err := h.configs.Upsert(r.Context(), uuid.New().String(), "jira", string(configJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, cfg)
}

func (h *integrationHandler) jiraTest(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "jira not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	client := jira.NewClient(jiraCfg)
	if err := client.TestConnection(r.Context()); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *integrationHandler) jiraSync(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "jira")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "jira not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var jiraCfg jira.Config
	if err := json.Unmarshal([]byte(cfg.Config), &jiraCfg); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	result, err := jira.Sync(r.Context(), jiraCfg, h.tasks)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	_ = h.configs.TouchSyncTime(r.Context(), "jira")
	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) jiraDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "jira"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "jira not configured")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Gmail ─────────────────────────────────────────────────────────────────────

func (h *integrationHandler) gmailGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"connected":      true,
		"enabled":        cfg.Enabled,
		"email":          stored.Email,
		"labels":         stored.Labels,
		"last_synced_at": cfg.LastSyncedAt,
	})
}

func (h *integrationHandler) gmailAuth(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Gmail OAuth not configured on this server")
		return
	}
	state := gmail.GenerateState()
	redirectURI := h.cfg.AppURL + "/api/v1/integrations/gmail/callback"
	authURL := gmail.AuthURL(h.cfg.GmailClientID, redirectURI, state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *integrationHandler) gmailCallback(w http.ResponseWriter, r *http.Request) {
	code  := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if !gmail.ConsumeState(state) {
		respondError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}

	redirectURI := h.cfg.AppURL + "/api/v1/integrations/gmail/callback"
	stored, err := gmail.ExchangeCode(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, redirectURI, code)
	if err != nil {
		respondError(w, http.StatusBadGateway, "token exchange failed: "+err.Error())
		return
	}

	email, err := gmail.FetchEmail(r.Context(), stored.AccessToken)
	if err == nil {
		stored.Email = email
	}

	configJSON, err := json.Marshal(stored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), "gmail", string(configJSON)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect back to the frontend settings page
	http.Redirect(w, r, h.cfg.FrontendURL+"/settings/integrations/gmail?connected=1", http.StatusTemporaryRedirect)
}

func (h *integrationHandler) gmailUpdateLabels(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body struct {
		Labels []string `json:"labels"`
	}
	if err := decode(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}
	stored.Labels = body.Labels

	configJSON, err := json.Marshal(stored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, updated)
}

func (h *integrationHandler) gmailSync(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Gmail OAuth not configured on this server")
		return
	}

	cfg, err := h.configs.Get(r.Context(), "gmail")
	if errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusBadRequest, "gmail not connected")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var stored gmail.StoredToken
	if err := json.Unmarshal([]byte(cfg.Config), &stored); err != nil {
		respondError(w, http.StatusInternalServerError, "malformed config")
		return
	}

	result, err := gmail.Sync(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, &stored, h.tasks)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Persist refreshed token if it changed
	configJSON, _ := json.Marshal(stored)
	_, _ = h.configs.UpdateConfig(r.Context(), "gmail", string(configJSON))
	_ = h.configs.TouchSyncTime(r.Context(), "gmail")

	respond(w, http.StatusOK, result)
}

func (h *integrationHandler) gmailDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), "gmail"); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gmail not connected")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
