package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
	prompt := fmt.Sprintf(`Today is %s (%s). Convert this quick note into ONE task and extract its details.
Resolve relative days ("today", "tomorrow", weekday names) to an actual YYYY-MM-DD date.
Treat "next week" as the upcoming Monday.
Read times like "9am", "1pm", "14:30" and durations like "30min", "1h", "45m".
Return JSON with exactly these keys:
"title": concise, action-oriented, WITHOUT any date/time/duration words
"planned_date": "YYYY-MM-DD" if a day is implied, else ""
"time_estimate_minutes": integer minutes if a duration is implied, else 0
"reminder_at": "YYYY-MM-DDTHH:MM:SS" if a specific time is implied, else ""
"tags": array, ONLY from this allowed JSON list (else []): %s

Example 1:
Today 2026-01-05 (Monday), Note: "submit taxes friday 2pm 45min #finance"
{"title":"Submit taxes","planned_date":"2026-01-09","time_estimate_minutes":45,"reminder_at":"2026-01-09T14:00:00","tags":["finance"]}

Example 2:
Today 2026-01-05 (Monday), Note: "buy milk"
{"title":"Buy milk","planned_date":"","time_estimate_minutes":0,"reminder_at":"","tags":[]}

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
Return JSON: "summary" (a clear task title, max 10 words, start with a verb), "time_estimate_minutes" (integer best-guess effort, 0 if unsure).

Example:
Subject: "Server down"
Body: "The main database server keeps dropping connections."
Output: {"summary": "Troubleshoot database server connection drops", "time_estimate_minutes": 30}

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
	// IMPORTANT: do NOT put example tag values in this prompt. Small models
	// (e.g. llama3.2:3b, qwen2.5:1.5b) copy example tags like "work" verbatim
	// into their answer; those aren't in the user's set, so filterAllowed strips
	// everything and the UI shows nothing. Instead, restate the allowed list and
	// forbid inventing tags.
	prompt := fmt.Sprintf(`You assign tags to a task. You may ONLY use tags from this exact list, copied verbatim (keep their exact spelling and capitalisation): %s
Do NOT invent tags or output any word that is not in that list.
Include a tag only if it clearly applies. It is perfectly fine to return none.
Return JSON: {"tags": [...]} where every element is copied EXACTLY from the allowed list above. Use an empty array if none apply.
Task title: %q
Task notes: %q`, string(availJSON), body.Title, clip(body.Notes, 800))

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
	prompt := fmt.Sprintf(`Break this task into 3 to 6 concrete, ordered subtasks.
Each subtask must be 8 words or fewer.
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

// aiPlanDay builds a soft schedule for the day's tasks around fixed calendar
// events. Division of labour: the model does what a small LLM is reliable at —
// pick a focused ORDER and estimate each task's effort — and Go does the clock
// arithmetic (packing tasks into the free gaps between events). Asking a 3B model
// to compute non-overlapping times around meetings is unreliable; deterministic
// slot-packing here is not. The result carries a per-task "roughly_at" (HH:MM),
// which is the daily board's primary sort key, so the plan actually lands on the
// board instead of just being shown once.
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
		respond(w, http.StatusOK, map[string]any{"available": true, "order": []string{}, "schedule": []any{}, "note": ""})
		return
	}
	tj, _ := json.Marshal(body.Tasks)
	ej, _ := json.Marshal(body.Events)
	prompt := fmt.Sprintf(`Plan a focused order for today's tasks and estimate how long each one takes.
Front-load important work and quick wins, group similar work together, and keep it realistic.
Each task lists its current "minutes" estimate; 0 means unknown — estimate it yourself.
Do NOT reorder the calendar events; they are fixed commitments to plan around.
You MUST use the exact ID strings from the Tasks list.
Return JSON:
"order": array of task ids in the suggested order, every id exactly once.
"estimates": object mapping each task id to an integer minutes estimate between 15 and 120.
"note": one short sentence of guidance.

Example JSON output:
{"order":["a3","a1"],"estimates":{"a3":45,"a1":30},"note":"Tackle the writing first, before your afternoon meetings."}

Tasks: %s
Fixed calendar events: %s`, string(tj), string(ej))

	var out struct {
		Order     []string       `json:"order"`
		Estimates map[string]int `json:"estimates"`
		Note      string         `json:"note"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Keep only valid ids, then append any the model dropped (so nothing is lost).
	type taskIn = struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Minutes int    `json:"minutes"`
	}
	byID := map[string]taskIn{}
	for _, t := range body.Tasks {
		byID[t.ID] = t
	}
	seen := map[string]bool{}
	order := make([]string, 0, len(body.Tasks))
	for _, id := range out.Order {
		if _, ok := byID[id]; ok && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	for _, t := range body.Tasks {
		if !seen[t.ID] {
			order = append(order, t.ID)
		}
	}

	// Lay out soft start times by packing tasks into the gaps around events.
	// Event start/end arrive as local "HH:MM" already resolved by the client —
	// the browser knows the user's timezone, the server's container may not, so
	// converting zones here would skew busy blocks for UTC-stored feeds.
	busy := parseBusyIntervals(body.Events)
	const dayStart = 9 * 60 // 09:00 working-window start (end is advisory; tasks may run past 18:00)
	cursor := dayStart
	type slot struct {
		ID        string `json:"id"`
		RoughlyAt string `json:"roughly_at"`
		Minutes   int    `json:"minutes"`
	}
	schedule := make([]slot, 0, len(order))
	for _, id := range order {
		t := byID[id]
		dur := t.Minutes
		if dur <= 0 {
			dur = out.Estimates[id]
		}
		if dur < 15 {
			dur = 30 // default a sane block when neither side gave one
		}
		if dur > 240 {
			dur = 240
		}
		cursor = roundUp5(cursor)
		cursor = nextFreeSlot(cursor, dur, busy)
		schedule = append(schedule, slot{ID: id, RoughlyAt: fmtHM(cursor), Minutes: dur})
		cursor += dur
	}

	respond(w, http.StatusOK, map[string]any{
		"available": true,
		"order":     order,
		"schedule":  schedule,
		"note":      strings.TrimSpace(out.Note),
	})
}

// parseBusyIntervals turns the request's timed events into [startMin,endMin]
// windows (minutes since local midnight). Each start/end is a local "HH:MM"
// (the client drops all-day events and resolves zones before sending).
// Unparseable or zero-length entries are skipped.
func parseBusyIntervals(events []struct {
	Title string `json:"title"`
	Start string `json:"start"`
	End   string `json:"end"`
}) [][2]int {
	out := [][2]int{}
	for _, ev := range events {
		s, okS := parseHM(ev.Start)
		e, okE := parseHM(ev.End)
		if !okS || !okE || e <= s {
			continue
		}
		out = append(out, [2]int{s, e})
	}
	return out
}

// parseHM reads a local "HH:MM" (24-hour) string into minutes since midnight.
func parseHM(s string) (int, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// nextFreeSlot advances `start` forward until a [start, start+dur) window clears
// every busy interval, then returns that start (minutes since midnight).
func nextFreeSlot(start, dur int, busy [][2]int) int {
	for {
		moved := false
		for _, b := range busy {
			if start < b[1] && start+dur > b[0] { // overlaps a busy block
				start = roundUp5(b[1])
				moved = true
			}
		}
		if !moved {
			return start
		}
	}
}

func roundUp5(min int) int {
	if r := min % 5; r != 0 {
		min += 5 - r
	}
	return min
}

func fmtHM(min int) string {
	if min > 23*60+59 {
		min = 23*60 + 59 // clamp inside the day
	}
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
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
Keep every bullet point under 15 words. Do not invent work that is not explicitly listed.
Completed tasks: %s
Objectives: %s`, string(cj), string(oj))

	var out struct {
		Wins       []string `json:"wins"`
		Challenges []string `json:"challenges"`
		NextFocus  string   `json:"next_focus"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out, ai.Options{Temperature: 0.6}); err != nil {
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
Keep every question under 15 words. Do not invent work that is not explicitly listed.
Done: %s
Not done: %s`, string(dj), string(uj))

	var out struct {
		Prompts []string `json:"prompts"`
	}
	if err := ai.JSON(r.Context(), cfg, prompt, &out, ai.Options{Temperature: 0.6}); err != nil {
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
- Do not summarize or shorten. You must preserve every detail.
- Turn run-on or pasted text into proper sentences and short paragraphs.
- Use "*" bullet points for any list of items, and "1." numbered steps for a sequence.
- Output ONLY the reformatted Markdown. No preamble, no explanation, no code fences.

Notes:
%s`, clip(body.Notes, 4000))

	// Reformatting preserves every detail, so the output can be as long as the
	// input — raise the token cap well above the 512 default to avoid truncation.
	out, err := ai.Text(r.Context(), cfg, prompt, ai.Options{Temperature: 0.2, NumPredict: 2048})
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
	if len(s) <= n {
		return s
	}
	// Back up to a rune boundary so we never split a multibyte UTF-8 character.
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
