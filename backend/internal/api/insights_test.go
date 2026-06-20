package api

import "testing"

import "github.com/clevercode/sempa/internal/db"

func TestComputeTimeInsights(t *testing.T) {
	t.Run("below threshold is unavailable", func(t *testing.T) {
		ins := computeTimeInsights([]db.TimeSample{
			{Estimate: 10, Actual: 20},
			{Estimate: 10, Actual: 20},
		})
		if ins.Available {
			t.Fatalf("expected unavailable with %d samples", ins.Samples)
		}
	})

	t.Run("global multiplier is the median ratio", func(t *testing.T) {
		// Ratios: 2.0, 2.0, 4.0 -> median 2.0
		ins := computeTimeInsights([]db.TimeSample{
			{Estimate: 10, Actual: 20},
			{Estimate: 10, Actual: 20},
			{Estimate: 10, Actual: 40},
		})
		if !ins.Available {
			t.Fatal("expected available")
		}
		if ins.GlobalMultiplier != 2.0 {
			t.Fatalf("global = %v, want 2.0", ins.GlobalMultiplier)
		}
	})

	t.Run("per-tag multipliers require minimum samples", func(t *testing.T) {
		ins := computeTimeInsights([]db.TimeSample{
			{Estimate: 10, Actual: 30, Tags: []string{"email"}},
			{Estimate: 10, Actual: 30, Tags: []string{"email"}},
			{Estimate: 10, Actual: 30, Tags: []string{"email"}},
			{Estimate: 10, Actual: 15, Tags: []string{"rare"}}, // only 1 — excluded
		})
		var email *tagMultiplier
		for i := range ins.Tags {
			if ins.Tags[i].Tag == "email" {
				email = &ins.Tags[i]
			}
			if ins.Tags[i].Tag == "rare" {
				t.Fatal("tag with <3 samples should be excluded")
			}
		}
		if email == nil || email.Multiplier != 3.0 {
			t.Fatalf("email multiplier = %v, want 3.0", email)
		}
	})
}

func TestMedian(t *testing.T) {
	if got := median([]float64{3, 1, 2}); got != 2 {
		t.Fatalf("odd median = %v, want 2", got)
	}
	if got := median([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Fatalf("even median = %v, want 2.5", got)
	}
	if got := median(nil); got != 0 {
		t.Fatalf("empty median = %v, want 0", got)
	}
}

func TestPickSimilarSamples(t *testing.T) {
	samples := []db.TimeSample{
		{Title: "a", Tags: []string{"email"}},
		{Title: "b", Tags: []string{"deep"}},
		{Title: "c", Tags: []string{"email"}},
		{Title: "d", Tags: []string{"admin"}},
	}
	// Tag match plus padding to a minimum of 3 for usable context.
	got := pickSimilarSamples(samples, []string{"email"}, 8)
	if len(got) != 3 {
		t.Fatalf("got %d similar, want 3 (2 matched + 1 padded)", len(got))
	}
	if got[0].Title != "a" || got[1].Title != "c" {
		t.Fatalf("tag matches should come first, got %q,%q", got[0].Title, got[1].Title)
	}
}
