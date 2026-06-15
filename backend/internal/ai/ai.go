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

// generate calls Ollama's /api/chat once (non-streamed). When jsonMode is set,
// Ollama is asked to emit strictly valid JSON (format:json), which makes parsing
// reliable even with small models. We use /api/chat rather than /api/generate
// because some models (e.g. recent llama3.2 builds) only expose a chat template
// and return "does not support generate" otherwise — chat works for both.
func generate(ctx context.Context, c Config, prompt string, jsonMode bool) (string, error) {
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

// Text returns a plain-text completion.
func Text(ctx context.Context, c Config, prompt string) (string, error) {
	return generate(ctx, c, prompt, false)
}

// JSON runs a completion in JSON mode and unmarshals it into v. It tolerates a
// model that wraps the object in prose by extracting the outermost {...}.
func JSON(ctx context.Context, c Config, prompt string, v any) error {
	s, err := generate(ctx, c, prompt, true)
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
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
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
