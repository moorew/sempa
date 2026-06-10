package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func mkTask(t *testing.T, s *TaskStore, title string, tags []string) {
	t.Helper()
	if _, err := s.Create(context.Background(), CreateTaskParams{
		ID: uuid.New().String(), Title: title, Status: "backlog", Position: 1, Tags: tags,
	}); err != nil {
		t.Fatalf("create %q: %v", title, err)
	}
}

func titles(ts []Task) map[string]bool {
	m := map[string]bool{}
	for _, t := range ts {
		m[t.Title] = true
	}
	return m
}

func TestTaskSearch_TextAndTags(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mkTask(t, s, "Finish work report", []string{"work"})
	mkTask(t, s, "Buy groceries", []string{"personal"})
	mkTask(t, s, "Work and personal errands", []string{"work", "personal"})
	mkTask(t, s, "Untagged note about report", nil)

	// Text only — matches title across tagged + untagged.
	got, err := s.Search(ctx, "report", nil, false, 50)
	if err != nil {
		t.Fatal(err)
	}
	if m := titles(got); !m["Finish work report"] || !m["Untagged note about report"] || len(got) != 2 {
		t.Errorf("text search 'report' = %v", m)
	}

	// Tag OR — any of {work, personal}.
	got, _ = s.Search(ctx, "", []string{"work", "personal"}, false, 50)
	if len(got) != 3 {
		t.Errorf("OR {work,personal} want 3, got %d", len(got))
	}

	// Tag AND — must have both.
	got, _ = s.Search(ctx, "", []string{"work", "personal"}, true, 50)
	if m := titles(got); len(got) != 1 || !m["Work and personal errands"] {
		t.Errorf("AND {work,personal} = %v", titles(got))
	}

	// Tag + text combined.
	got, _ = s.Search(ctx, "errands", []string{"work"}, false, 50)
	if m := titles(got); len(got) != 1 || !m["Work and personal errands"] {
		t.Errorf("text+tag = %v", titles(got))
	}

	// Single tag.
	got, _ = s.Search(ctx, "", []string{"personal"}, false, 50)
	if len(got) != 2 {
		t.Errorf("single tag 'personal' want 2, got %d", len(got))
	}

	// Empty query + empty tags → nothing.
	got, _ = s.Search(ctx, "", nil, false, 50)
	if len(got) != 0 {
		t.Errorf("empty search want 0, got %d", len(got))
	}
}
