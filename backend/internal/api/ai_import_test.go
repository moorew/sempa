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
	if rec.Steps[0].Title == "" {
		t.Errorf("step title not derived: %#v", rec.Steps[0])
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
