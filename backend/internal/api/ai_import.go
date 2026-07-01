package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/clevercode/sempa/internal/ai"
	"github.com/clevercode/sempa/internal/integrations/unfurl"
)

// AI Import: turn a URL or pasted text into ONE actionable task whose steps
// become subtasks, plus a companion list (ingredients → groceries, packing
// items, materials, …). Recipe is the flagship case but the pipeline is generic
// and auto-detects the content type. When the page carries structured
// schema.org/Recipe JSON-LD we parse it deterministically (far more reliable
// than a small model) and skip the LLM entirely.

// importResult is the structured extraction the client turns into a parent task
// (+ steps as subtasks) and a new list (+ items).
type importResult struct {
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Notes    string   `json:"notes"`
	Steps    []string `json:"steps"`
	ListName string   `json:"list_name"`
	Items    []string `json:"items"`
}

func (h *integrationHandler) aiImport(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.aiCfg(r.Context())
	if !ok {
		aiUnavailable(w)
		return
	}
	var body struct {
		URL  string `json:"url"`
		Text string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	body.URL = strings.TrimSpace(body.URL)
	body.Text = strings.TrimSpace(body.Text)
	if body.URL == "" && body.Text == "" {
		respondError(w, http.StatusBadRequest, "url or text required")
		return
	}

	content := body.Text
	sourceURL := ""

	if body.URL != "" {
		doc, _, err := unfurl.FetchContent(r.Context(), body.URL)
		if err != nil {
			respondError(w, http.StatusBadGateway, fmt.Sprintf("couldn't fetch that URL: %v", err))
			return
		}
		sourceURL = body.URL
		// Deterministic fast path: structured recipe data. Only trust it if it
		// actually yielded steps or ingredients; otherwise fall through to the LLM.
		if rec, ok := extractRecipeJSONLD(doc); ok && (len(rec.Steps) > 0 || len(rec.Items) > 0) {
			respond(w, http.StatusOK, importResponse(rec, sourceURL))
			return
		}
		content = htmlToText(doc)
	}

	if strings.TrimSpace(content) == "" {
		respondError(w, http.StatusUnprocessableEntity, "no readable content found at that URL")
		return
	}

	res, err := aiExtractImport(r.Context(), cfg, content)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, http.StatusOK, importResponse(res, sourceURL))
}

// aiExtractImport asks the local model to structure freeform content into a
// task + steps + a companion gather/shopping list, auto-detecting the type.
func aiExtractImport(ctx context.Context, cfg ai.Config, content string) (importResult, error) {
	prompt := fmt.Sprintf(`Turn the content below into ONE actionable task with ordered steps, plus a companion list of physical things to buy or gather.
First detect "type": one of "recipe", "trip", "project", "generic".
Return JSON with exactly these keys:
"type": one of the four above
"title": a short, action-oriented task title (e.g. "Smoke a brisket", "Pack for Lisbon"). No trailing punctuation.
"notes": one short sentence of context, or ""
"steps": array of the ordered steps/instructions, each a short imperative sentence
"list_name": a short name for the companion list (e.g. "Brisket — groceries", "Lisbon — packing"), or ""
"items": array of physical things to buy or gather (ingredients, supplies, packing items), each copied with its quantity if given. Empty array if there are none.
Only use steps and items supported by the content — do not invent any.

Content:
%s`, clip(content, 6000))

	var out importResult
	// Instructions + ingredient lists can be long; raise the cap above the 512 default.
	if err := ai.JSON(ctx, cfg, prompt, &out, ai.Options{Temperature: 0.2, NumPredict: 1536}); err != nil {
		return importResult{}, err
	}
	return out, nil
}

// importResponse normalises + shapes an extraction into the API response the
// client consumes (it then creates the task, subtasks, list and items).
func importResponse(res importResult, sourceURL string) map[string]any {
	typ := strings.ToLower(strings.TrimSpace(res.Type))
	switch typ {
	case "recipe", "trip", "project", "generic":
	default:
		typ = "generic"
	}
	title := strings.TrimSpace(res.Title)
	if title == "" {
		title = "Imported task"
	}
	steps := cleanList(res.Steps, 40)
	items := cleanList(res.Items, 100)
	listName := strings.TrimSpace(res.ListName)
	if listName == "" && len(items) > 0 {
		listName = title + " — items"
	}
	return map[string]any{
		"available":  true,
		"type":       typ,
		"title":      clip(title, 200),
		"notes":      strings.TrimSpace(res.Notes),
		"steps":      steps,
		"list_name":  listName,
		"items":      items,
		"source_url": sourceURL,
	}
}

// ── schema.org/Recipe (JSON-LD) extraction ───────────────────────────────────

var jsonLDRe = regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)

// extractRecipeJSONLD scans a page's JSON-LD blocks for a schema.org/Recipe and
// maps the first one found. Handles a single object, an array, or an @graph.
func extractRecipeJSONLD(doc string) (importResult, bool) {
	for _, m := range jsonLDRe.FindAllStringSubmatch(doc, -1) {
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		var parsed any
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			continue
		}
		if rec, ok := findRecipe(parsed); ok {
			return rec, true
		}
	}
	return importResult{}, false
}

func findRecipe(v any) (importResult, bool) {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if r, ok := findRecipe(e); ok {
				return r, true
			}
		}
	case map[string]any:
		if g, ok := t["@graph"]; ok {
			if r, ok := findRecipe(g); ok {
				return r, true
			}
		}
		if isRecipeType(t["@type"]) {
			return mapRecipe(t), true
		}
	}
	return importResult{}, false
}

// isRecipeType reports whether a JSON-LD @type (a string or array of strings)
// includes "Recipe".
func isRecipeType(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, "Recipe")
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.EqualFold(s, "Recipe") {
				return true
			}
		}
	}
	return false
}

func mapRecipe(m map[string]any) importResult {
	title := jsonLDString(m["name"])
	ingredients := jsonLDStrings(m["recipeIngredient"])
	if len(ingredients) == 0 {
		ingredients = jsonLDStrings(m["ingredients"]) // legacy key
	}
	listName := ""
	if title != "" {
		listName = title + " — groceries"
	}
	return importResult{
		Type:     "recipe",
		Title:    title,
		Notes:    clip(jsonLDString(m["description"]), 240),
		Steps:    jsonLDInstructions(m["recipeInstructions"]),
		ListName: listName,
		Items:    ingredients,
	}
}

// jsonLDString reads a JSON-LD text value: a plain string, or the first element
// of an array (some producers wrap single values).
func jsonLDString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(html.UnescapeString(t))
	case []any:
		if len(t) > 0 {
			return jsonLDString(t[0])
		}
	}
	return ""
}

func jsonLDStrings(v any) []string {
	out := []string{}
	switch t := v.(type) {
	case string:
		if s := strings.TrimSpace(html.UnescapeString(t)); s != "" {
			out = append(out, s)
		}
	case []any:
		for _, e := range t {
			if s := jsonLDString(e); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// jsonLDInstructions normalises recipeInstructions, which appears as: a plain
// (possibly newline-separated) string; an array of strings; an array of
// HowToStep objects ({text|name}); or HowToSection objects wrapping
// itemListElement.
func jsonLDInstructions(v any) []string {
	out := []string{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			for _, line := range strings.Split(html.UnescapeString(t), "\n") {
				if s := strings.TrimSpace(line); s != "" {
					out = append(out, s)
				}
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			if strings.EqualFold(jsonLDString(t["@type"]), "HowToSection") {
				walk(t["itemListElement"])
				return
			}
			if s := jsonLDString(t["text"]); s != "" {
				out = append(out, s)
				return
			}
			if s := jsonLDString(t["name"]); s != "" {
				out = append(out, s)
			}
		}
	}
	walk(v)
	return out
}

// ── HTML → text (LLM fallback) ───────────────────────────────────────────────

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
	anyTagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	wsRe          = regexp.MustCompile(`\s+`)
)

// htmlToText strips a document to readable text for the model fallback: drop
// script/style, remove tags, unescape entities, collapse whitespace.
func htmlToText(doc string) string {
	doc = scriptStyleRe.ReplaceAllString(doc, " ")
	doc = anyTagRe.ReplaceAllString(doc, " ")
	doc = html.UnescapeString(doc)
	return strings.TrimSpace(wsRe.ReplaceAllString(doc, " "))
}
