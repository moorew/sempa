package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/clevercode/sempa/internal/backup"
	"github.com/clevercode/sempa/internal/config"
	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/gmail"
)

// backupDriveType is the integration_configs key for the backup Drive token.
const backupDriveType = backup.DriveConfigType

func driveRedirectURI(cfg config.Config) string {
	return cfg.AppURL + "/api/v1/backup/drive/callback"
}

// driveStateSep separates the CSRF token from the (base64url) deep-link return
// prefix inside the OAuth `state` param. `state` is opaque to Google and
// round-trips unchanged, so it's a safe carrier. The separator is outside both
// the hex (CSRF) and base64url alphabets, so it can't collide.
const driveStateSep = "~"

// driveReturnPrefix returns the deep-link scheme a native client must be bounced
// back to after consent, or "" for web (which uses the popup + HTML result page).
// Android opens consent in a Chrome Custom Tab and can't make use of a "Return to
// Sempa" web page — it needs a com.clevercode.sempa:// deep link to re-foreground
// the app, mirroring the Google sign-in flow.
func driveReturnPrefix(r *http.Request) string {
	if r.URL.Query().Get("native") == "true" {
		return "com.clevercode.sempa://drive-backup"
	}
	return ""
}

// splitDriveState separates the CSRF token from an optional base64url return
// prefix. The prefix is whitelisted to our own deep-link schemes so a tampered
// state can never turn the callback into an open redirect.
func splitDriveState(state string) (csrf, returnPrefix string) {
	csrf = state
	if i := strings.IndexByte(state, driveStateSep[0]); i >= 0 {
		csrf = state[:i]
		if b, err := base64.RawURLEncoding.DecodeString(state[i+1:]); err == nil {
			returnPrefix = string(b)
		}
	}
	switch returnPrefix {
	case "com.clevercode.sempa://drive-backup", "sempa://drive-backup":
	default:
		returnPrefix = ""
	}
	return
}

// driveAuth starts the Google consent flow for the drive.file scope.
func (h *backupHandler) driveAuth(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GmailClientID == "" {
		respondError(w, http.StatusServiceUnavailable, "Google OAuth is not configured on this server")
		return
	}
	state := gmail.GenerateState()
	if prefix := driveReturnPrefix(r); prefix != "" {
		state += driveStateSep + base64.RawURLEncoding.EncodeToString([]byte(prefix))
	}
	consentURL := gmail.AuthURLForScopes(h.cfg.GmailClientID, driveRedirectURI(h.cfg), state, gmail.ScopeDriveFile)
	http.Redirect(w, r, consentURL, http.StatusTemporaryRedirect)
}

// driveCallback exchanges the code and stores the Drive token.
func (h *backupHandler) driveCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	csrf, returnPrefix := splitDriveState(r.URL.Query().Get("state"))
	if !gmail.ConsumeState(csrf) {
		h.driveOAuthDone(w, returnPrefix, false, "This sign-in link expired. Please try connecting again.")
		return
	}
	stored, err := gmail.ExchangeCode(r.Context(), h.cfg.GmailClientID, h.cfg.GmailClientSecret, driveRedirectURI(h.cfg), code)
	if err != nil {
		h.driveOAuthDone(w, returnPrefix, false, "Token exchange failed: "+err.Error())
		return
	}
	if email, err := gmail.FetchEmail(r.Context(), stored.AccessToken); err == nil {
		stored.Email = email
	}
	cfgJSON, _ := json.Marshal(stored)
	if _, err := h.configs.Upsert(r.Context(), uuid.New().String(), backupDriveType, string(cfgJSON)); err != nil {
		h.driveOAuthDone(w, returnPrefix, false, "Could not save the connection. Please try again.")
		return
	}
	// A fresh token clears the reason the last run failed, so retire that stale
	// result — otherwise the "backups need reconnecting" banner keeps nagging
	// after the user has already done exactly what it asked.
	_ = h.store.ClearLastResult(r.Context())
	h.driveOAuthDone(w, returnPrefix, true, "")
}

// driveOAuthDone finishes the consent flow. Native clients (Android) get a deep
// link that re-foregrounds the app so the backup page can re-check status; web
// and desktop get the self-contained HTML result page.
func (h *backupHandler) driveOAuthDone(w http.ResponseWriter, returnPrefix string, ok bool, errMsg string) {
	if returnPrefix != "" {
		status := "error"
		if ok {
			status = "connected"
		}
		q := url.Values{"drive": {status}, "redirect": {"/settings/backup"}}
		w.Header().Set("Location", returnPrefix+"?"+q.Encode())
		w.WriteHeader(http.StatusFound)
		return
	}
	writeOAuthResultPage(w, h.cfg.FrontendURL, ok, errMsg)
}

// writeOAuthResultPage renders a small self-contained page after the Drive OAuth
// redirect. It works for every client: a web popup is notified via postMessage
// and auto-closes; everything else shows a clear "you can return to Sempa"
// message instead of dumping the user into the web app's login screen.
func writeOAuthResultPage(w http.ResponseWriter, frontendURL string, ok bool, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	title := "Google Drive connected"
	icon := "✓"
	color := "#16a34a"
	body := "You can close this window and return to Sempa."
	if !ok {
		title = "Couldn’t connect Google Drive"
		icon = "✕"
		color = "#dc2626"
		body = htmlEscape(errMsg)
	}
	backURL := htmlEscape(frontendURL) + "/settings/backup?drive=" + map[bool]string{true: "connected", false: "error"}[ok]
	msg := "sempa-drive-" + map[bool]string{true: "connected", false: "error"}[ok]
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
 body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;background:#faf7f2;color:#2b2620;
 display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0;padding:24px}
 .card{max-width:380px;text-align:center;background:#fff;border-radius:18px;padding:36px 28px;
 box-shadow:0 8px 30px rgba(0,0,0,.08)}
 .icon{width:56px;height:56px;border-radius:50%%;background:%s;color:#fff;font-size:28px;line-height:56px;margin:0 auto 16px}
 h1{font-size:18px;margin:0 0 8px} p{font-size:14px;color:#6b6358;margin:0 0 20px;line-height:1.5}
 a{display:inline-block;background:#b3592e;color:#fff;text-decoration:none;padding:10px 20px;border-radius:10px;font-size:14px;font-weight:600}
</style></head><body>
<div class="card">
 <div class="icon">%s</div>
 <h1>%s</h1>
 <p>%s</p>
 <a href="%s">Return to Sempa</a>
</div>
<script>
 try { if (window.opener) { window.opener.postMessage('%s','*'); setTimeout(function(){window.close();}, 400); } } catch (e) {}
</script>
</body></html>`, htmlEscape(title), color, icon, htmlEscape(title), body, backURL, msg)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func (h *backupHandler) driveStatus(w http.ResponseWriter, r *http.Request) {
	c, err := h.configs.Get(r.Context(), backupDriveType)
	if errors.Is(err, db.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var tok gmail.StoredToken
	_ = json.Unmarshal([]byte(c.Config), &tok)
	// Probe the token so the UI can prompt a reconnect when it's expired/revoked
	// (Google "Testing" apps expire refresh tokens after 7 days) instead of
	// showing "connected" while every backup silently fails with invalid_grant.
	needsReconnect := false
	if _, terr := driveTokenFunc(h.configs, h.cfg)(r.Context()); terr != nil && errors.Is(terr, gmail.ErrReauthRequired) {
		needsReconnect = true
	}
	respond(w, http.StatusOK, map[string]any{"connected": true, "email": tok.Email, "needs_reconnect": needsReconnect})
}

func (h *backupHandler) driveDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.configs.Delete(r.Context(), backupDriveType); err != nil && !errors.Is(err, db.ErrNotFound) {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusNoContent, nil)
}

// driveTokenFunc resolves a Google Drive access token from the stored token.
func driveTokenFunc(configs *db.IntegrationConfigStore, cfg config.Config) backup.DriveTokenFunc {
	return backup.DriveTokenResolver(configs, cfg.GmailClientID, cfg.GmailClientSecret)
}
