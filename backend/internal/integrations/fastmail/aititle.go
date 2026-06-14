package fastmail

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

// SECURITY — accepted request-forgery (SSRF) finding [go/request-forgery].
//
// ListModels and ImproveTitle make an HTTP request to a base URL that the
// instance owner configures (Settings → Integrations, or OLLAMA_BASE_URL). This
// is intentional: the feature exists to talk to a self-hosted model server,
// which by design lives at an internal/loopback address (e.g.
// http://ollama:11434 or http://localhost:11434) — so restricting requests to
// public hosts (the usual SSRF mitigation) would break it. The URL is settable
// only by the authenticated owner (who already controls the server), never by
// untrusted input, and the API layer validates it is a well-formed http(s) URL.
// The residual risk is accepted; the CodeQL alert is dismissed with this
// justification. See SECURITY.md and the README "AI task-title cleanup" notes.

// ModelInfo is a model available on the Ollama host plus its on-disk size in
// bytes, so the settings UI can show how much space each model uses.
type ModelInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ListModelsDetailed returns the models on the Ollama host at baseURL with their
// sizes. It doubles as a reachability check (used by the settings UI's "Test"
// button). A short timeout keeps the settings page responsive.
func ListModelsDetailed(ctx context.Context, baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("ollama base URL not set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(out.Models))
	for _, m := range out.Models {
		models = append(models, ModelInfo{Name: m.Name, Size: m.Size})
	}
	return models, nil
}

// ListModels returns just the model names (reachability + name list).
func ListModels(ctx context.Context, baseURL string) ([]string, error) {
	models, err := ListModelsDetailed(ctx, baseURL)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(models))
	for _, m := range models {
		names = append(names, m.Name)
	}
	return names, nil
}

// PullProgress is one streamed update while a model downloads.
type PullProgress struct {
	Status    string `json:"status"`
	Completed int64  `json:"completed"`
	Total     int64  `json:"total"`
}

// PullModel streams a model download from the Ollama host, invoking onProgress
// for each update. It blocks until the pull completes, errors, or ctx is
// cancelled. No client timeout — large models take minutes.
func PullModel(ctx context.Context, baseURL, model string, onProgress func(PullProgress)) error {
	if baseURL == "" {
		return fmt.Errorf("ollama base URL not set")
	}
	if model == "" {
		return fmt.Errorf("model name required")
	}
	reqBody, _ := json.Marshal(map[string]any{"model": model, "stream": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/pull", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{} // no timeout — pulls can take several minutes
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("ollama pull returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var line struct {
			Status    string `json:"status"`
			Completed int64  `json:"completed"`
			Total     int64  `json:"total"`
			Error     string `json:"error"`
		}
		if err := dec.Decode(&line); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if line.Error != "" {
			return fmt.Errorf("%s", line.Error)
		}
		if onProgress != nil {
			onProgress(PullProgress{Status: line.Status, Completed: line.Completed, Total: line.Total})
		}
	}
}

// DeleteModel removes a model from the Ollama host to free disk space.
func DeleteModel(ctx context.Context, baseURL, model string) error {
	if baseURL == "" {
		return fmt.Errorf("ollama base URL not set")
	}
	if model == "" {
		return fmt.Errorf("model name required")
	}
	// Send both "model" (current API) and "name" (older API) for compatibility.
	reqBody, _ := json.Marshal(map[string]any{"model": model, "name": model})
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/api/delete", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("ollama delete returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// ImproveTitle uses a local Ollama model to turn an email subject into a
// concise action-oriented task title. Falls back to the stripped subject on
// any error or if Ollama is not configured.
func ImproveTitle(ctx context.Context, ollamaBaseURL, model, subject string) string {
	if ollamaBaseURL == "" || subject == "" {
		return subject
	}
	if model == "" {
		model = "qwen2.5:1.5b"
	}

	prompt := fmt.Sprintf(
		"Convert this email subject into a brief, action-oriented task title. "+
			"Maximum 8 words. Start with a verb. Remove newsletter boilerplate, "+
			"company names, and urgency language. Return ONLY the task title.\n\n"+
			"Subject: %q\n\nTask title:", subject)

	body, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return subject
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return subject
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return subject
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return subject
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return subject
	}

	title := strings.TrimSpace(result.Response)
	title = strings.Trim(title, `"'`)
	if title == "" {
		return subject
	}
	return title
}
