package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/unfurl"
	"github.com/clevercode/sempa/internal/notify"
)

type notificationHandler struct {
	configs  *db.IntegrationConfigStore
	pushSubs *db.PushSubStore
	vapidPub string
}

// getSettings returns the current notification settings, falling back to
// defaults when nothing has been saved yet. The timezone is always returned
// resolved to the *effective* home zone (env default if unset) so clients have a
// concrete IANA name to compare the device against for travel detection.
func (h *notificationHandler) getSettings(w http.ResponseWriter, r *http.Request) {
	st := notify.LoadSettings(r.Context(), h.configs)
	if st.Timezone == "" {
		st.Timezone = db.ServerLocation().String()
	}
	respond(w, http.StatusOK, redactSettings(st))
}

// redactSettings blanks the webhook secret before settings leave the server.
// Settings are readable by every household member, and the admin's form sends
// the blank back unchanged (see putSettings).
func redactSettings(st notify.Settings) notify.Settings {
	st.WebhookAuthSet = st.Webhook.AuthValue != ""
	st.Webhook.AuthValue = ""
	return st
}

// putSettings persists the notification settings document.
func (h *notificationHandler) putSettings(w http.ResponseWriter, r *http.Request) {
	var s notify.Settings
	if err := decode(r, &s); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// The client only ever sees a redacted secret, so a blank auth value means
	// "keep the stored one" — but only while the endpoint and header are
	// unchanged, so the secret can't be re-pointed at a new host.
	if s.Webhook.AuthValue == "" {
		prev := notify.LoadSettings(r.Context(), h.configs).Webhook
		if prev.Endpoint == s.Webhook.Endpoint && prev.AuthHeader == s.Webhook.AuthHeader {
			s.Webhook.AuthValue = prev.AuthValue
		}
	}
	s.WebhookAuthSet = false
	raw, err := json.Marshal(s)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode settings")
		return
	}
	if _, err := h.configs.Upsert(r.Context(), newID(), "notifications", string(raw)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	// Apply a timezone change immediately so recurrence + digest pick up the new
	// home zone without a redeploy (empty resolves back to the env default).
	db.SetServerLocation(db.ResolveLocation(s.Timezone))
	if s.Timezone == "" {
		s.Timezone = db.ServerLocation().String()
	}
	respond(w, http.StatusOK, redactSettings(s))
}

// vapidKey hands the browser the application server public key it needs to call
// pushManager.subscribe().
func (h *notificationHandler) vapidKey(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, map[string]string{"key": h.vapidPub})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Platform string `json:"platform"`
}

// subscribe stores a browser Web Push subscription (PushSubscription.toJSON()).
func (h *notificationHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if err := decode(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		respondError(w, http.StatusBadRequest, "endpoint and keys are required")
		return
	}
	// The server POSTs to this URL on every reminder, so it must be a real push
	// service: https on a public address, never something inside the network.
	if _, verr := unfurl.ValidatePublicURL(req.Endpoint); verr != nil || !strings.HasPrefix(strings.ToLower(req.Endpoint), "https://") {
		respondError(w, http.StatusBadRequest, "endpoint must be a public https push service URL")
		return
	}
	sub, err := h.pushSubs.Upsert(newID(), req.Endpoint, req.Keys.P256dh, req.Keys.Auth, req.Platform, ownerID(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}
	respond(w, http.StatusOK, sub)
}

// unsubscribe removes a Web Push subscription by endpoint.
func (h *notificationHandler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decode(r, &req); err != nil || req.Endpoint == "" {
		respondError(w, http.StatusBadRequest, "endpoint is required")
		return
	}
	if err := h.pushSubs.DeleteByEndpointOwned(req.Endpoint, ownerID(r)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testWebhook validates the generic webhook configuration as entered in the
// settings form (tests the posted config, not necessarily the saved one).
func (h *notificationHandler) testWebhook(w http.ResponseWriter, r *http.Request) {
	var cfg notify.WebhookConfig
	if err := decode(r, &cfg); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// The form holds a redacted secret; reuse the stored one for an unchanged
	// endpoint so "Send test" works without re-typing it.
	if cfg.AuthValue == "" {
		prev := notify.LoadSettings(r.Context(), h.configs).Webhook
		if prev.Endpoint == cfg.Endpoint && prev.AuthHeader == cfg.AuthHeader {
			cfg.AuthValue = prev.AuthValue
		}
	}
	if err := notify.TestWebhook(cfg); err != nil {
		respondError(w, http.StatusBadGateway, "webhook test failed: "+err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]bool{"ok": true})
}
