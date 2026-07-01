package api

import (
	"strings"
	"testing"
)

func TestExtractRecipeJSONLD_HowToSteps(t *testing.T) {
	doc := `<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Recipe","name":"Smoked Brisket",
 "description":"Low and slow.",
 "recipeIngredient":["1 whole brisket","2 tbsp salt","2 tbsp pepper"],
 "recipeInstructions":[
   {"@type":"HowToStep","text":"Trim the brisket."},
   {"@type":"HowToStep","text":"Season generously."},
   {"@type":"HowToStep","text":"Smoke at 225F for 12 hours."}
 ]}
</script></head><body>hi</body></html>`

	rec, ok := extractRecipeJSONLD(doc)
	if !ok {
		t.Fatal("expected a recipe to be extracted")
	}
	if rec.Title != "Smoked Brisket" {
		t.Errorf("title = %q", rec.Title)
	}
	if len(rec.Items) != 3 || rec.Items[0] != "1 whole brisket" {
		t.Errorf("ingredients = %#v", rec.Items)
	}
	if len(rec.Steps) != 3 || rec.Steps[2].Detail != "Smoke at 225F for 12 hours." {
		t.Errorf("steps = %#v", rec.Steps)
	}
	// cleanSteps should derive a short title from each step's detail.
	cleaned := cleanSteps(rec.Steps, 40)
	if cleaned[0].Title == "" || wordCount(cleaned[0].Title) > 6 {
		t.Errorf("derived title bad: %q", cleaned[0].Title)
	}
	if rec.ListName != "Smoked Brisket — groceries" {
		t.Errorf("list name = %q", rec.ListName)
	}
}

func TestExtractRecipeJSONLD_Graph_ArrayType_StringInstructions(t *testing.T) {
	// Recipe nested in an @graph, @type as an array, and instructions as a
	// single newline-separated string — all common real-world variants.
	doc := `<script type="application/ld+json">
{"@graph":[
  {"@type":"WebPage","name":"ignore me"},
  {"@type":["Recipe","Thing"],"name":"Pancakes",
   "recipeIngredient":["2 cups flour","1 cup milk"],
   "recipeInstructions":"Mix dry.\nAdd milk.\nCook on griddle."}
]}
</script>`

	rec, ok := extractRecipeJSONLD(doc)
	if !ok {
		t.Fatal("expected a recipe from @graph")
	}
	if rec.Title != "Pancakes" {
		t.Errorf("title = %q", rec.Title)
	}
	if len(rec.Steps) != 3 || rec.Steps[0].Detail != "Mix dry." {
		t.Errorf("steps = %#v", rec.Steps)
	}
	if len(rec.Items) != 2 {
		t.Errorf("ingredients = %#v", rec.Items)
	}
}

func TestExtractRecipeJSONLD_NoRecipe(t *testing.T) {
	doc := `<script type="application/ld+json">{"@type":"Article","name":"Not a recipe"}</script>`
	if _, ok := extractRecipeJSONLD(doc); ok {
		t.Fatal("did not expect a recipe")
	}
}

func TestCleanSteps_ShortensTitlesKeepsDetail(t *testing.T) {
	in := []importStep{
		// detail-only, two sentences → short title, full detail preserved.
		{Detail: "Mix the dry rub ingredients in a bowl until they are completely combined. Apply generously onto the pork shoulder."},
		// model dumped the whole step into the title → shorten, detail gets the full text.
		{Title: "Mix the dry rub ingredients in a bowl until combined"},
		// a genuinely short, distinct title → kept as-is.
		{Title: "Preheat oven", Detail: "Preheat the oven to 225F."},
		// tiny step → detail not duplicated.
		{Title: "Rest", Detail: "Rest"},
	}
	out := cleanSteps(in, 40)
	if len(out) != 4 {
		t.Fatalf("len = %d: %#v", len(out), out)
	}
	if wordCount(out[0].Title) > 6 {
		t.Errorf("title too long: %q", out[0].Title)
	}
	if strings.HasSuffix(out[0].Title, ".") {
		t.Errorf("title has trailing punctuation: %q", out[0].Title)
	}
	if !strings.Contains(out[0].Detail, "Apply generously") {
		t.Errorf("detail lost the full instruction: %q", out[0].Detail)
	}
	if wordCount(out[1].Title) > 6 || out[1].Detail == "" {
		t.Errorf("dumped-title step not fixed: %#v", out[1])
	}
	if out[2].Title != "Preheat oven" {
		t.Errorf("short distinct title changed: %q", out[2].Title)
	}
	if out[3].Detail != "" {
		t.Errorf("tiny step duplicated detail: %q", out[3].Detail)
	}
}

func TestHTMLToText(t *testing.T) {
	got := htmlToText(`<html><head><style>.x{}</style><script>var a=1;</script></head>
<body><h1>Title</h1><p>Hello &amp; welcome</p></body></html>`)
	if strings.Contains(got, "var a") || strings.Contains(got, ".x{}") {
		t.Errorf("script/style not stripped: %q", got)
	}
	if !strings.Contains(got, "Title") || !strings.Contains(got, "Hello & welcome") {
		t.Errorf("text/entities lost: %q", got)
	}
}
