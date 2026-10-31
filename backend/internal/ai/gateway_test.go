package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// apertureModels is the shape of Aperture's GET /v1/models, as measured against
// a live gateway (moorew/clerk docs/model-access.md): one model listed twice
// under different providers, one of them passthrough.
const apertureModels = `{"object":"list","data":[
 {"id":"claude-opus-5","supported_endpoints":["anthropic_messages"],"context_window_tokens":200000,
  "metadata":{"provider":{"id":"anthropic-passthrough","requires_client_auth":true}}},
 {"id":"claude-opus-5","supported_endpoints":["anthropic_messages"],"context_window_tokens":200000,
  "metadata":{"provider":{"id":"anthropic-api","requires_client_auth":false}}},
 {"id":"qwen3:14b","supported_endpoints":["openai_chat"],"metadata":{"provider":{"id":"ollama"}}},
 {"id":"gpt-research","supported_endpoints":["openai_responses"],"metadata":{"provider":{"id":"openai"}}},
 {"id":""}
]}`

func TestParseGatewayModels(t *testing.T) {
	ms, err := parseGatewayModels([]byte(apertureModels))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]GatewayModel{}
	for _, m := range ms {
		byID[m.ID] = m
	}
	if len(ms) != 4 {
		t.Fatalf("got %d models, want 4 (empty id skipped): %+v", len(ms), ms)
	}
	// A repeated id is disambiguated by provider, so a request can reach the
	// copy Aperture holds the key for instead of the passthrough one.
	api, ok := byID["anthropic-api/claude-opus-5"]
	if !ok || !api.Usable || api.Wire != WireAnthropicMessages || api.Name != "claude-opus-5" || api.ContextWindow != 200000 {
		t.Errorf("anthropic-api copy = %+v", api)
	}
	pt := byID["anthropic-passthrough/claude-opus-5"]
	if pt.Usable || !strings.Contains(pt.Reason, "passthrough") {
		t.Errorf("passthrough copy should be unusable with a reason, got %+v", pt)
	}
	if q := byID["qwen3:14b"]; !q.Usable || q.Wire != WireOpenAIChat || q.Provider != "ollama" {
		t.Errorf("qwen = %+v", q)
	}
	if r := byID["gpt-research"]; r.Usable || r.Wire != "" || !strings.Contains(r.Reason, "openai_responses") {
		t.Errorf("responses-only model should be unusable, got %+v", r)
	}
	// Usable first.
	if !ms[0].Usable || !ms[1].Usable || ms[2].Usable || ms[3].Usable {
		t.Errorf("usable models should sort first: %+v", ms)
	}
}

func TestParseGatewayModelsPlainOpenAI(t *testing.T) {
	// A gateway with no Aperture extras still lists usable chat models.
	ms, err := parseGatewayModels([]byte(`{"data":[{"id":"m1","metadata":{"provider":"acme"}},{"id":"m2"}]}`))
	if err != nil || len(ms) != 2 {
		t.Fatalf("ms=%+v err=%v", ms, err)
	}
	for _, m := range ms {
		if !m.Usable || m.Wire != WireOpenAIChat {
			t.Errorf("%+v should be a usable chat model", m)
		}
	}
	var m1 GatewayModel
	for _, m := range ms {
		if m.ID == "m1" {
			m1 = m
		}
	}
	if m1.Provider != "acme" {
		t.Errorf("string provider not read: %+v", m1)
	}
	if _, err := parseGatewayModels([]byte(`not json`)); err == nil {
		t.Error("garbage should be an error")
	}
}

func TestPickWire(t *testing.T) {
	cases := map[string]struct {
		in   []string
		want string
	}{
		"none listed":   {nil, WireOpenAIChat},
		"chat":          {[]string{"openai_chat"}, WireOpenAIChat},
		"both":          {[]string{"anthropic_messages", "openai_chat"}, WireOpenAIChat},
		"messages only": {[]string{"anthropic_messages"}, WireAnthropicMessages},
		"responses":     {[]string{"openai_responses"}, ""},
		"embeddings":    {[]string{"openai_embeddings"}, ""},
	}
	for name, c := range cases {
		if got := pickWire(c.in); got != c.want {
			t.Errorf("%s: pickWire(%v) = %q, want %q", name, c.in, got, c.want)
		}
	}
}

// fakeAperture serves /v1/models from apertureModels and records the requests.
type fakeAperture struct {
	chat, messages func(w http.ResponseWriter, body map[string]any, r *http.Request)
	chatCalls      atomic.Int32
	messagesCalls  atomic.Int32
}

func (f *fakeAperture) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(apertureModels))
		case "/v1/chat/completions":
			f.chatCalls.Add(1)
			f.chat(w, body, r)
		case "/v1/messages":
			f.messagesCalls.Add(1)
			f.messages(w, body, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGatewayChatJSONFallsBackToMinimalRequest(t *testing.T) {
	f := &fakeAperture{}
	f.chat = func(w http.ResponseWriter, body map[string]any, _ *http.Request) {
		// Refuse the optional knobs the way a strict provider does.
		if _, ok := body["response_format"]; ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"response_format is not supported"}}`))
			return
		}
		if _, ok := body["temperature"]; ok {
			t.Errorf("minimal retry still carried temperature: %v", body)
		}
		msgs := body["messages"].([]any)
		if first := msgs[0].(map[string]any); first["role"] != "system" {
			t.Errorf("JSON mode should lead with the system instruction, got %v", first)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Sure:\n{\"title\":\"Buy milk\"}"}}]}`))
	}
	srv := f.server(t)
	var out struct {
		Title string `json:"title"`
	}
	cfg := Config{Provider: ProviderAperture, BaseURL: srv.URL + "/", Model: "qwen3:14b"}
	if err := JSON(context.Background(), cfg, "Return JSON with a title", &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != "Buy milk" {
		t.Errorf("title = %q", out.Title)
	}
	if n := f.chatCalls.Load(); n != 2 {
		t.Errorf("chat calls = %d, want 2 (full, then minimal)", n)
	}
}

func TestGatewayAnthropicMessages(t *testing.T) {
	f := &fakeAperture{}
	f.messages = func(w http.ResponseWriter, body map[string]any, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		if r.Header.Get("x-api-key") != "" || r.Header.Get("Authorization") != "" {
			t.Error("Sempa must not send a credential; tailnet identity is the auth")
		}
		if body["model"] != "anthropic-api/claude-opus-5" {
			t.Errorf("model = %v", body["model"])
		}
		if body["system"] == nil {
			t.Error("JSON mode should set a system instruction")
		}
		if mt, _ := body["max_tokens"].(float64); mt < 1024 {
			t.Errorf("max_tokens = %v, want the gateway floor", body["max_tokens"])
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"thinking","thinking":"..."},{"type":"text","text":"{\"ok\":"},{"type":"text","text":"true}"}]}`))
	}
	srv := f.server(t)
	var out struct {
		OK bool `json:"ok"`
	}
	cfg := Config{Provider: ProviderAperture, BaseURL: srv.URL, Model: "anthropic-api/claude-opus-5"}
	if err := JSON(context.Background(), cfg, "Return JSON", &out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || f.messagesCalls.Load() != 1 || f.chatCalls.Load() != 0 {
		t.Errorf("ok=%v messages=%d chat=%d", out.OK, f.messagesCalls.Load(), f.chatCalls.Load())
	}
}

func TestGatewayRetriesTransientAndReportsErrors(t *testing.T) {
	f := &fakeAperture{}
	f.chat = func(w http.ResponseWriter, body map[string]any, _ *http.Request) {
		if f.chatCalls.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"Ready"}]}}]}`))
	}
	srv := f.server(t)
	got, err := Text(context.Background(), Config{Provider: ProviderAperture, BaseURL: srv.URL, Model: "qwen3:14b"}, "hi")
	if err != nil || got != "Ready" {
		t.Fatalf("got %q err %v", got, err)
	}

	// A refusal is passed on with the gateway's own words.
	f2 := &fakeAperture{}
	f2.chat = func(w http.ResponseWriter, _ map[string]any, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"model \"qwen3:14b\" is not granted to user \"x@example.com\""}}`))
	}
	srv2 := f2.server(t)
	_, err = Text(context.Background(), Config{Provider: ProviderAperture, BaseURL: srv2.URL, Model: "qwen3:14b"}, "hi")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "not granted") {
		t.Errorf("err = %v", err)
	}
}

func TestConfigReady(t *testing.T) {
	if (Config{BaseURL: "http://x"}).Ready() != true {
		t.Error("ollama with a URL is ready (model has a default)")
	}
	if (Config{Provider: ProviderAperture, BaseURL: "http://x"}).Ready() {
		t.Error("a gateway with no model picked is not ready")
	}
	if !(Config{Provider: ProviderAperture, BaseURL: "http://x", Model: "m"}).Ready() {
		t.Error("gateway with URL and model is ready")
	}
}

func TestClipErr(t *testing.T) {
	if got := clipErr([]byte(`{"error":{"message":"nope"}}`)); got != "nope" {
		t.Errorf("object error: %q", got)
	}
	if got := clipErr([]byte(`{"error":"flat"}`)); got != "flat" {
		t.Errorf("string error: %q", got)
	}
	if got := clipErr([]byte(strings.Repeat("é", 400))); len([]rune(got)) != 301 {
		t.Errorf("long body should clip to 300 runes + ellipsis, got %d", len([]rune(got)))
	}
}
