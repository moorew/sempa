package ai

// Aperture (Tailscale's AI gateway) support.
//
// Aperture sits on the tailnet and routes each request to whichever provider
// serves the model asked for. It authenticates the caller by tailnet identity,
// so Sempa sends no key: the server's own node is who Aperture sees. It speaks
// two wire formats, per model, and says which in GET /v1/models:
//
//   openai_chat         POST /v1/chat/completions
//   anthropic_messages  POST /v1/messages
//
// Models Aperture holds a provider key for need nothing from us. A model it
// passes through to someone's own account ("requires_client_auth") needs that
// account's key on every request; Sempa deliberately holds none, so those are
// listed as unusable with the fix (give the key to Aperture instead). This
// mirrors moorew/clerk, measured against a live gateway (docs/model-access.md
// there).
//
// SECURITY: like the Ollama client, this requests an owner-configured URL that
// is expected to be on the tailnet. That is the accepted request-forgery
// finding documented in fastmail/aititle.go: only the authenticated owner can
// set it, and the API validates it is a well-formed http(s) URL.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Wire formats a gateway model can be reached on.
const (
	WireOpenAIChat        = "openai_chat"
	WireAnthropicMessages = "anthropic_messages"
)

// GatewayModel is one model the gateway will serve this server's identity.
type GatewayModel struct {
	// ID is what to send as "model". Normally the bare model id; when the same
	// id is listed under two providers (say a passthrough copy and one Aperture
	// holds the key for) each copy is "provider/id", which is also how Aperture
	// is asked for a particular provider's copy.
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider,omitempty"`
	Wire          string `json:"wire"`
	ContextWindow int    `json:"context_window,omitempty"`
	Usable        bool   `json:"usable"`
	Reason        string `json:"reason,omitempty"`
}

// ── Transport ───────────────────────────────────────────────────────────────

// tailnetResolver asks Tailscale's own DNS (quad-100, served by the local
// tailscaled) directly. Used only when the system resolver can't find a
// *.ts.net name, which happens inside containers whose resolv.conf doesn't
// point at MagicDNS even though the tailnet itself is reachable.
var tailnetResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		d := net.Dialer{Timeout: 2 * time.Second}
		return d.DialContext(ctx, "udp", "100.100.100.100:53")
	},
}

func dialWithTailnetFallback(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := d.DialContext(ctx, network, addr)
	if err == nil {
		return conn, nil
	}
	host, port, splitErr := net.SplitHostPort(addr)
	var dnsErr *net.DNSError
	if splitErr != nil || !errors.As(err, &dnsErr) || !strings.HasSuffix(strings.TrimSuffix(host, "."), ".ts.net") {
		return nil, err
	}
	addrs, lerr := tailnetResolver.LookupIPAddr(ctx, host)
	if lerr != nil || len(addrs) == 0 {
		return nil, err // the original error says more than ours would
	}
	// TLS still verifies against the URL's host name, not this address.
	return d.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
}

var gatewayClient = &http.Client{
	// Cloud models answer in seconds, but a big model on a busy provider (or a
	// self-hosted one behind the gateway) can take a while; stay generous.
	Timeout: 90 * time.Second,
	Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         dialWithTailnetFallback,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 4,
	},
}

func gatewayURL(base, path string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/") + path
}

// ── Discovery ───────────────────────────────────────────────────────────────

type modelCache struct {
	at     time.Time
	models []GatewayModel
	err    error
}

var (
	cacheMu sync.Mutex
	cache   = map[string]modelCache{}
)

const (
	cacheOK   = 60 * time.Second
	cacheFail = 20 * time.Second
)

// GatewayModels returns the models at baseURL, from a short cache. A failed
// lookup is cached briefly too, so an unreachable gateway doesn't make every
// settings load wait out a timeout.
func GatewayModels(ctx context.Context, baseURL string) ([]GatewayModel, error) {
	key := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	cacheMu.Lock()
	c, ok := cache[key]
	cacheMu.Unlock()
	if ok && ((c.err == nil && time.Since(c.at) < cacheOK) || (c.err != nil && time.Since(c.at) < cacheFail)) {
		return c.models, c.err
	}
	return RefreshGatewayModels(ctx, baseURL)
}

// RefreshGatewayModels asks the gateway directly (bypassing the cache) and
// stores the answer.
func RefreshGatewayModels(ctx context.Context, baseURL string) ([]GatewayModel, error) {
	key := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	models, err := listGatewayModels(ctx, key)
	cacheMu.Lock()
	cache[key] = modelCache{at: time.Now(), models: models, err: err}
	cacheMu.Unlock()
	return models, err
}

func listGatewayModels(ctx context.Context, baseURL string) ([]GatewayModel, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("gateway URL not set")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL(baseURL, "/v1/models"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := gatewayClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned %d: %s", resp.StatusCode, clipErr(raw))
	}
	return parseGatewayModels(raw)
}

// parseGatewayModels reads an OpenAI-style model list, plus Aperture's
// per-model extras (supported_endpoints, metadata.provider). Extras that are
// missing or shaped differently are tolerated, so any OpenAI-compatible
// gateway lists at least its model ids.
func parseGatewayModels(raw []byte) ([]GatewayModel, error) {
	var payload struct {
		Data []struct {
			ID                  string          `json:"id"`
			SupportedEndpoints  []string        `json:"supported_endpoints"`
			ContextWindowTokens int             `json:"context_window_tokens"`
			Metadata            json.RawMessage `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("gateway sent an unreadable model list: %w", err)
	}
	type prov struct {
		id   string
		auth bool
	}
	readProvider := func(meta json.RawMessage) prov {
		var m struct {
			Provider json.RawMessage `json:"provider"`
		}
		if len(meta) == 0 || json.Unmarshal(meta, &m) != nil || len(m.Provider) == 0 {
			return prov{}
		}
		var obj struct {
			ID                 string `json:"id"`
			RequiresClientAuth bool   `json:"requires_client_auth"`
		}
		if json.Unmarshal(m.Provider, &obj) == nil && obj.ID != "" {
			return prov{obj.ID, obj.RequiresClientAuth}
		}
		var s string
		if json.Unmarshal(m.Provider, &s) == nil {
			return prov{id: s}
		}
		return prov{}
	}

	seen := map[string]int{}
	for _, e := range payload.Data {
		if e.ID != "" {
			seen[e.ID]++
		}
	}
	out := make([]GatewayModel, 0, len(payload.Data))
	for _, e := range payload.Data {
		if e.ID == "" {
			continue
		}
		p := readProvider(e.Metadata)
		m := GatewayModel{ID: e.ID, Name: e.ID, Provider: p.id, ContextWindow: e.ContextWindowTokens}
		if seen[e.ID] > 1 && p.id != "" {
			m.ID = p.id + "/" + e.ID
		}
		m.Wire = pickWire(e.SupportedEndpoints)
		switch {
		case m.Wire == "":
			m.Reason = "Only offered as " + strings.Join(e.SupportedEndpoints, ", ") + ", which Sempa doesn't use."
		case p.auth:
			m.Reason = "Runs in passthrough, so it needs your own provider key on every request. " +
				"Sempa doesn't hold keys: add the key to Aperture as a regular provider and the model shows up as usable."
		default:
			m.Usable = true
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Usable != out[j].Usable {
			return out[i].Usable
		}
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// pickWire prefers chat completions (it has a JSON mode), then Anthropic
// messages. No list at all means a plain OpenAI-compatible gateway: assume chat.
func pickWire(supported []string) string {
	if len(supported) == 0 {
		return WireOpenAIChat
	}
	hasMessages := false
	for _, s := range supported {
		s = strings.ToLower(s)
		if strings.Contains(s, "chat") {
			return WireOpenAIChat
		}
		if strings.Contains(s, "messages") {
			hasMessages = true
		}
	}
	if hasMessages {
		return WireAnthropicMessages
	}
	return ""
}

// wireFor finds the wire for c.Model, refreshing the list once if the model
// isn't in it (newly granted, say). Unknown models are tried as chat: the
// gateway's own error then says which endpoint it wanted.
func wireFor(ctx context.Context, c Config) string {
	find := func(ms []GatewayModel) string {
		for _, m := range ms {
			if m.ID == c.Model || m.Name == c.Model {
				return m.Wire
			}
		}
		return ""
	}
	ms, err := GatewayModels(ctx, c.BaseURL)
	if w := find(ms); err == nil && w != "" {
		return w
	}
	if ms, err = RefreshGatewayModels(ctx, c.BaseURL); err == nil {
		if w := find(ms); w != "" {
			return w
		}
	}
	return WireOpenAIChat
}

// ── Generation ──────────────────────────────────────────────────────────────

const jsonInstruction = "Reply with one valid JSON value and nothing else: no prose, no code fences."

// gatewayMaxTokens gives cloud models more room than the CPU-sized local caps:
// some spend part of the budget reasoning before they answer, and a cap sized
// for a 1.5B model would cut them off mid-answer.
func gatewayMaxTokens(o Options) int {
	if o.NumPredict < 1024 {
		return 1024
	}
	return o.NumPredict
}

func gatewayGenerate(ctx context.Context, c Config, prompt string, jsonMode bool, opts Options) (string, error) {
	if strings.TrimSpace(c.Model) == "" {
		return "", fmt.Errorf("no gateway model chosen")
	}
	if wireFor(ctx, c) == WireAnthropicMessages {
		return anthropicMessages(ctx, c, prompt, jsonMode, opts)
	}
	return openAIChat(ctx, c, prompt, jsonMode, opts)
}

func openAIChat(ctx context.Context, c Config, prompt string, jsonMode bool, opts Options) (string, error) {
	messages := []map[string]string{}
	if jsonMode {
		messages = append(messages, map[string]string{"role": "system", "content": jsonInstruction})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})
	full := map[string]any{
		"model":       c.Model,
		"messages":    messages,
		"temperature": opts.Temperature,
		"max_tokens":  gatewayMaxTokens(opts),
	}
	if jsonMode {
		full["response_format"] = map[string]string{"type": "json_object"}
	}
	// Providers disagree on the optional knobs (some reasoning models refuse a
	// temperature or want max_completion_tokens; not every one has JSON mode).
	// If the full request is refused, the bare one is accepted everywhere.
	minimal := map[string]any{"model": c.Model, "messages": messages}

	raw, err := postGateway(ctx, gatewayURL(c.BaseURL, "/v1/chat/completions"), nil, full, minimal)
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("gateway sent an unexpected reply: %s", clipErr(raw))
	}
	return strings.TrimSpace(contentText(out.Choices[0].Message.Content)), nil
}

// contentText reads a chat message's content, which is normally a string but
// may be a list of typed parts.
func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "" || p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return ""
}

func anthropicMessages(ctx context.Context, c Config, prompt string, jsonMode bool, opts Options) (string, error) {
	messages := []map[string]string{{"role": "user", "content": prompt}}
	full := map[string]any{
		"model":       c.Model,
		"max_tokens":  gatewayMaxTokens(opts),
		"temperature": opts.Temperature,
		"messages":    messages,
	}
	minimal := map[string]any{"model": c.Model, "max_tokens": gatewayMaxTokens(opts), "messages": messages}
	if jsonMode {
		full["system"] = jsonInstruction
		minimal["system"] = jsonInstruction
	}
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	raw, err := postGateway(ctx, gatewayURL(c.BaseURL, "/v1/messages"), headers, full, minimal)
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gateway sent an unexpected reply: %s", clipErr(raw))
	}
	var b strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// postGateway sends body (falling back to fallback once if the gateway refuses
// it as a bad request) and retries a transient 429/5xx once.
func postGateway(ctx context.Context, url string, headers map[string]string, body, fallback map[string]any) ([]byte, error) {
	raw, status, err := postOnce(ctx, url, headers, body)
	if err == nil && isTransient(status) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		// Jitter, not cryptography: only to keep retries from lining up.
		case <-time.After(time.Second + time.Duration(rand.IntN(400))*time.Millisecond): // #nosec G404
		}
		raw, status, err = postOnce(ctx, url, headers, body)
	}
	if err == nil && status == http.StatusBadRequest && fallback != nil {
		raw, status, err = postOnce(ctx, url, headers, fallback)
	}
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		// Aperture's errors are precise ("model X is available via
		// anthropic_messages, not openai_chat, for user Y"), so pass them on.
		return nil, fmt.Errorf("gateway returned %d: %s", status, clipErr(raw))
	}
	return raw, nil
}

func postOnce(ctx context.Context, url string, headers map[string]string, body map[string]any) ([]byte, int, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := gatewayClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func isTransient(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// clipErr pulls the message out of a JSON error body if there is one, and
// keeps whatever it returns short enough for a settings line.
func clipErr(raw []byte) string {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	msg := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &e) == nil && len(e.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		var s string
		switch {
		case json.Unmarshal(e.Error, &obj) == nil && obj.Message != "":
			msg = obj.Message
		case json.Unmarshal(e.Error, &s) == nil && s != "":
			msg = s
		}
	}
	if r := []rune(msg); len(r) > 300 {
		msg = string(r[:300]) + "…"
	}
	return msg
}
