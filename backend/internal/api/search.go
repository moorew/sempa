package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/clevercode/sempa/internal/db"
)

type searchHandler struct {
	tasks      *db.TaskStore
	objectives *db.ObjectiveStore
	plans      *db.DailyPlanStore
	reviews    *db.WeekReviewStore
}

// journalHit is a flattened journal match (a daily plan or a week review).
type journalHit struct {
	Kind    string `json:"kind"` // "daily" | "week"
	Date    string `json:"date"` // plan_date or week_start
	Snippet string `json:"snippet"`
}

type searchResults struct {
	Tasks      []db.Task      `json:"tasks"`
	Objectives []db.Objective `json:"objectives"`
	Journal    []journalHit   `json:"journal"`
}

const searchLimit = 50

// search runs a global search over tasks, objectives and journal entries.
// Query params: q (text), tags (comma-separated), match=any|all.
// Tag filtering applies to tasks only (objectives/journal have no tags), so
// when tags are supplied those sections are omitted.
func (h *searchHandler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	matchAll := r.URL.Query().Get("match") == "all"

	var tags []string
	if raw := strings.TrimSpace(r.URL.Query().Get("tags")); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
	}

	out := searchResults{Tasks: []db.Task{}, Objectives: []db.Objective{}, Journal: []journalHit{}}
	if q == "" && len(tags) == 0 {
		respond(w, http.StatusOK, out)
		return
	}

	ctx := r.Context()
	if tasks, err := h.tasks.Search(ctx, q, tags, matchAll, searchLimit); err == nil {
		out.Tasks = tasks
	}

	// Objectives & journal have no tags, so only include them for a text query
	// with no tag filter.
	if q != "" && len(tags) == 0 {
		if objs, err := h.objectives.Search(ctx, q, searchLimit); err == nil {
			out.Objectives = objs
		}
		if plans, err := h.plans.Search(ctx, q, searchLimit); err == nil {
			for _, p := range plans {
				out.Journal = append(out.Journal, journalHit{
					Kind:    "daily",
					Date:    p.PlanDate,
					Snippet: snippet(q, firstNonEmpty(p.Intention, p.Reflection), p.Wins),
				})
			}
		}
		if reviews, err := h.reviews.Search(ctx, q, searchLimit); err == nil {
			for _, wr := range reviews {
				out.Journal = append(out.Journal, journalHit{
					Kind:    "week",
					Date:    wr.WeekStart,
					Snippet: snippet(q, wr.NextFocus, wr.Wins, wr.Challenges),
				})
			}
		}
	}

	respond(w, http.StatusOK, out)
}

func firstNonEmpty(vals ...*string) *string {
	for _, v := range vals {
		if v != nil && strings.TrimSpace(*v) != "" {
			return v
		}
	}
	return nil
}

// snippet returns a short excerpt around the first field that contains q (case-
// insensitive). Some fields are JSON arrays of strings (wins/challenges) — we
// flatten those to plain text first. Falls back to the first non-empty field.
func snippet(q string, fields ...*string) string {
	ql := strings.ToLower(q)
	var fallback string
	for _, f := range fields {
		if f == nil {
			continue
		}
		text := flattenMaybeJSON(*f)
		if text == "" {
			continue
		}
		if fallback == "" {
			fallback = text
		}
		if byteIdx := strings.Index(strings.ToLower(text), ql); byteIdx >= 0 {
			runeIdx := len([]rune(text[:byteIdx])) // byte → rune index (ASCII-exact)
			return excerpt([]rune(text), runeIdx, len([]rune(q)))
		}
	}
	return clipRunes(fallback, 140)
}

// flattenMaybeJSON renders a value that may be a JSON array of strings (e.g.
// wins/challenges) as a single readable line, else returns it as-is.
func flattenMaybeJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return strings.TrimSpace(strings.Join(arr, " · "))
		}
	}
	return s
}

// excerpt returns a window of runes around [idx, idx+matchLen) with ellipses.
func excerpt(runes []rune, idx, matchLen int) string {
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + matchLen + 80
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out = out + "…"
	}
	return strings.TrimSpace(out)
}

func clipRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
