package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/clevercode/sempa/internal/ai"
)

// AI-assist endpoints. Every handler resolves the instance's local model config;
// if AI is disabled or unconfigured it returns {"available": false} (HTTP 200)
// so the UI can quietly hide the feature rather than erroring. All inference runs
// on the self-hosted Ollama model — nothing is sent to a third party.

// aiCfg returns the resolved model config and whether AI is usable.
func (h *integrationHandler) aiCfg(ctx context.Context) (ai.Config, bool) {
	c := h.configs.ResolveAITitle(ctx, h.cfg.OllamaBaseURL, h.cfg.OllamaModel)
	cfg := ai.Config{BaseURL: c.BaseURL, Model: c.Model}
	return cfg, c.Enabled && cfg.Ready()
}

// unavailable writes the "AI off" response.
func aiUnavailable(w http.ResponseWriter) {
	respond(w, http.StatusOK, map[string]any{"available": false})
}

// aiQuickAdd parses a free-text capture into structured task fields.
func (h *integrationHandler) aiQuickAdd(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Text  string   `json:"text"`
		Today string   `json:"today"`
		Tags  []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		respondError(w, http.StatusBadRequest, "text required")
		return
	}
	weekday := ""
	if t, err := time.Parse("2006-01-02", body.Today); err == nil {
		weekday = t.Weekday().String()
	}
	tagsJSON, _ := json.Marshal(body.Tags)
	prompt := fmt.Sprintf(`Today is %s (%s). Convert this quick note into a single task.
Resolve relative days ("today", "tomorrow", "thursday", "next week") to an actual date.
Return JSON with keys: "title" (concise, action-oriented, no date/time words),
"planned_date" (YYYY-MM-DD if a day is implied else ""),
"time_estimate_minutes" (integer, 0 if none implied),
"reminder_at" (RFC3339 datetime if a specific time is implied else ""),
"tags" (array, choose only from this allowed JSON list, else empty): %s

Note: %q`, body.Today, weekday, string(tagsJSON), body.Text)

	var out struct {
		Title               string   `json:"title"`
		PlannedDate         string   `json:"planned_date"`
		TimeEstimateMinutes int      `json:"time_estimate_minutes"`
		ReminderAt          string   `json:"reminder_at"`
		Tags                []string `json:"tags"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	if strings.TrimSpace(out.Title) == "" {
		out.Title = strings.TrimSpace(body.Text)
	}
	respond(w, http.StatusOK, map[string]any{
		"available":             true,
		"title":                 out.Title,
		"planned_date":          out.PlannedDate,
		"time_estimate_minutes": out.TimeEstimateMinutes,
		"reminder_at":           out.ReminderAt,
		"tags":                  filterAllowed(out.Tags, body.Tags),
	})
}

// aiSummarizeTask turns an imported email/issue into a concise title + estimate.
func (h *integrationHandler) aiSummarizeTask(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	prompt := fmt.Sprintf(`Summarize this into an actionable task.
Return JSON: "summary" (a clear task title, max 10 words, start with a verb),
"time_estimate_minutes" (integer best-guess effort, 0 if unsure).

Subject: %q
Body: %q`, body.Title, clip(body.Body, 1500))

	var out struct {
		Summary             string `json:"summary"`
		TimeEstimateMinutes int    `json:"time_estimate_minutes"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"available":             true,
		"summary":               strings.TrimSpace(out.Summary),
		"time_estimate_minutes": out.TimeEstimateMinutes,
	})
}

// aiSuggestTags proposes tags for a task from the user's existing tag set.
func (h *integrationHandler) aiSuggestTags(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Title     string   `json:"title"`
		Notes     string   `json:"notes"`
		Available []string `json:"available_tags"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Available) == 0 {
		respond(w, http.StatusOK, map[string]any{"available": true, "tags": []string{}})
		return
	}
	availJSON, _ := json.Marshal(body.Available)
	prompt := fmt.Sprintf(`Choose the tags that fit this task, ONLY from the allowed JSON list.
Be generous: assign every tag that is even loosely relevant — most tasks get at least one.
Return JSON: "tags" (array, a subset of the allowed list; empty only if truly none apply).
Example — Task: "Pay rent", Allowed: ["finance","home","work"] → {"tags":["finance","home"]}
Allowed: %s
Task: %q
Notes: %q`, string(availJSON), body.Title, clip(body.Notes, 800))

	var out struct {
		Tags []string `json:"tags"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"available": true, "tags": filterAllowed(out.Tags, body.Available)})
}

// aiBreakdown splits a task into a few concrete subtasks.
func (h *integrationHandler) aiBreakdown(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Title string `json:"title"`
		Notes string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Title) == "" {
		respondError(w, http.StatusBadRequest, "title required")
		return
	}
	prompt := fmt.Sprintf(`Break this task into 3-6 concrete, ordered subtasks.
Return JSON: "subtasks" (array of short action strings, each starting with a verb).
Task: %q
Notes: %q`, body.Title, clip(body.Notes, 800))

	var out struct {
		Subtasks []string `json:"subtasks"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"available": true, "subtasks": cleanList(out.Subtasks, 8)})
}

// aiPlanDay suggests an order for the day's tasks (around any calendar events).
func (h *integrationHandler) aiPlanDay(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Date  string `json:"date"`
		Tasks []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Minutes int    `json:"minutes"`
		} `json:"tasks"`
		Events []struct {
			Title string `json:"title"`
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"events"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Tasks) == 0 {
		respond(w, http.StatusOK, map[string]any{"available": true, "order": []string{}, "note": ""})
		return
	}
	tj, _ := json.Marshal(body.Tasks)
	ej, _ := json.Marshal(body.Events)
	prompt := fmt.Sprintf(`Plan a focused order for today's tasks, accounting for fixed calendar events.
Front-load important/quick wins, group similar work, and keep it realistic.
Return JSON: "order" (array of task ids in the suggested order, every id exactly once),
"note" (one short sentence of guidance).
Tasks: %s
Events: %s`, string(tj), string(ej))

	var out struct {
		Order []string `json:"order"`
		Note  string   `json:"note"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Keep only valid ids, then append any the model dropped (so nothing is lost).
	valid := map[string]bool{}
	for _, t := range body.Tasks {
		valid[t.ID] = true
	}
	seen := map[string]bool{}
	order := make([]string, 0, len(body.Tasks))
	for _, id := range out.Order {
		if valid[id] && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	for _, t := range body.Tasks {
		if !seen[t.ID] {
			order = append(order, t.ID)
		}
	}
	respond(w, http.StatusOK, map[string]any{"available": true, "order": order, "note": strings.TrimSpace(out.Note)})
}

// aiWeeklyReview drafts wins / challenges / next-focus from the week's work.
func (h *integrationHandler) aiWeeklyReview(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Completed  []string `json:"completed"`
		Objectives []struct {
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"objectives"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	cj, _ := json.Marshal(body.Completed)
	oj, _ := json.Marshal(body.Objectives)
	prompt := fmt.Sprintf(`Draft a brief weekly review from the completed tasks and objectives.
Return JSON: "wins" (array of 2-4 short bullet strings),
"challenges" (array of 1-3 short bullet strings),
"next_focus" (one or two sentences).
Be specific and grounded in the data; don't invent work that isn't listed.
Completed tasks: %s
Objectives: %s`, string(cj), string(oj))

	var out struct {
		Wins       []string `json:"wins"`
		Challenges []string `json:"challenges"`
		NextFocus  string   `json:"next_focus"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"available":  true,
		"wins":       cleanList(out.Wins, 4),
		"challenges": cleanList(out.Challenges, 3),
		"next_focus": strings.TrimSpace(out.NextFocus),
	})
}

// aiReflectionPrompts offers a couple of context-aware shutdown questions.
func (h *integrationHandler) aiReflectionPrompts(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Done   []string `json:"done"`
		Undone []string `json:"undone"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	dj, _ := json.Marshal(body.Done)
	uj, _ := json.Marshal(body.Undone)
	prompt := fmt.Sprintf(`Write 2-3 short, thoughtful end-of-day reflection questions, tailored to what got
done and what didn't. Calm and constructive, not preachy.
Return JSON: "prompts" (array of question strings).
Done: %s
Not done: %s`, string(dj), string(uj))

	var out struct {
		Prompts []string `json:"prompts"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]any{"available": true, "prompts": cleanList(out.Prompts, 3)})
}

// aiTidyNotes cleans up free-form notes into tidy Markdown (paragraphs + lists)
// without changing the meaning or dropping any information.
func (h *integrationHandler) aiTidyNotes(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		Notes string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Notes) == "" {
		respondError(w, http.StatusBadRequest, "notes required")
		return
	}
	// Plain-text generation (not JSON) — the output is one freeform block, and a
	// small model reliably returns text but often gets a JSON key name wrong.
	prompt := fmt.Sprintf(`Reformat the notes below into clean, readable Markdown.
Rules:
- Keep ALL the information and any URLs exactly. Do not invent or remove facts.
- Turn run-on or pasted text into proper sentences and short paragraphs.
- Use "- " bullet points for any list of items, and "1." numbered steps for a sequence.
- Output ONLY the reformatted Markdown. No preamble, no explanation, no code fences.

Notes:
%s`, clip(body.Notes, 4000))

	out, err := ai.Text(r.Context(), cfg, prompt)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	cleaned := stripCodeFence(strings.TrimSpace(out))
	if cleaned == "" {
		cleaned = body.Notes
	}
	respond(w, http.StatusOK, map[string]any{"available": true, "notes": cleaned})
}

// ── helpers ────────────────────────────────────────────────────────────────

// filterAllowed keeps only items present in allowed (case-insensitive), preserving order.
func filterAllowed(items, allowed []string) []string {
	set := map[string]string{}
	for _, a := range allowed {
		set[strings.ToLower(strings.TrimSpace(a))] = a
	}
	out := []string{}
	seen := map[string]bool{}
	for _, it := range items {
		key := strings.ToLower(strings.TrimSpace(it))
		if canon, ok := set[key]; ok && !seen[key] {
			out = append(out, canon)
			seen[key] = true
		}
	}
	return out
}

// cleanList trims, drops empties, and caps the length.
func cleanList(items []string, max int) []string {
	out := []string{}
	for _, it := range items {
		s := strings.TrimSpace(it)
		if s == "" {
			continue
		}
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}

// stripCodeFence removes a leading/trailing ``` fence a model sometimes adds.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:] // drop the opening ``` (and any language tag)
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
