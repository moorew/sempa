// Package ai is a thin client over a local Ollama server, shared by the
// AI-assist features (quick-add parsing, task summaries, tag suggestions,
// subtask breakdown, day planning, weekly-review drafting, reflection prompts).
// Everything runs against the instance owner's self-hosted model — nothing
// leaves the server.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is the resolved Ollama endpoint + model for a request.
type Config struct {
	BaseURL string
	Model   string
}

// Ready reports whether AI calls can be made (a server URL is configured).
func (c Config) Ready() bool { return strings.TrimSpace(c.BaseURL) != "" }

// Options tunes a single generation. Lower temperature (near 0) suits structured
// extraction; higher (~0.6) suits the creative drafting features. NumPredict caps
// the generated tokens to keep CPU inference snappy and bound runaway output.
type Options struct {
	Temperature float64
	NumPredict  int
}

// defaultOptions are used when a caller passes no Options: cool and bounded, the
// right baseline for the extraction-style endpoints that dominate.
var defaultOptions = Options{Temperature: 0.2, NumPredict: 512}

func resolveOptions(opts []Options) Options {
	if len(opts) == 0 {
		return defaultOptions
	}
	o := opts[0]
	if o.NumPredict <= 0 {
		o.NumPredict = defaultOptions.NumPredict
	}
	return o
}

// generate calls Ollama's /api/chat once (non-streamed). When jsonMode is set,
// Ollama is asked to emit strictly valid JSON (format:json), which makes parsing
// reliable even with small models. We use /api/chat rather than /api/generate
// because some models (e.g. recent llama3.2 builds) only expose a chat template
// and return "does not support generate" otherwise — chat works for both.
func generate(ctx context.Context, c Config, prompt string, jsonMode bool, opts Options) (string, error) {
	if !c.Ready() {
		return "", fmt.Errorf("AI model server not configured")
	}
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = "qwen2.5:1.5b"
	}
	payload := map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   false,
		"options": map[string]any{
			"temperature": opts.Temperature,
			"num_predict": opts.NumPredict,
		},
	}
	if jsonMode {
		payload["format"] = "json"
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// CPU inference on a small model is slow-ish; allow generous headroom.
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Message.Content), nil
}

// Text returns a plain-text completion. An optional Options tunes temperature
// and the token cap; omit it for the cool, bounded defaults.
func Text(ctx context.Context, c Config, prompt string, opts ...Options) (string, error) {
	return generate(ctx, c, prompt, false, resolveOptions(opts))
}

// JSON runs a completion in JSON mode and unmarshals it into v. It tolerates a
// model that wraps the object in prose by extracting the outermost {...}. An
// optional Options tunes temperature and the token cap.
func JSON(ctx context.Context, c Config, prompt string, v any, opts ...Options) error {
	s, err := generate(ctx, c, prompt, true, resolveOptions(opts))
	if err != nil {
		return err
	}
	s = extractJSON(s)
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("model returned unparseable JSON: %w", err)
	}
	return nil
}

// extractJSON trims anything outside the first balanced {...} or [...] block so a
// chatty model can't break parsing.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return s
	}
	open := s[start]
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	// Track string state so braces/brackets inside a string value (e.g. a task
	// title like "fix render() {}") don't throw off the depth count.
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
