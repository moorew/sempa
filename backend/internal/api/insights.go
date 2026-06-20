package api

import (
	"context"
	"net/http"
	"sort"

	"github.com/clevercode/sempa/internal/db"
)

// Minimum data points before a multiplier is trustworthy enough to surface.
const minTimeSamples = 3

type tagMultiplier struct {
	Tag        string  `json:"tag"`
	Samples    int     `json:"samples"`
	Multiplier float64 `json:"multiplier"`
}

// timeInsights is the planned-vs-actual profile: an overall "you take Nx longer
// than you plan" multiplier plus per-tag multipliers. The headline numbers are
// plain statistics (transparent, work offline); the local AI layers nuance on
// top of these (see Phase 4 predict-time).
type recentSample struct {
	Title           string   `json:"title"`
	EstimateMinutes int64    `json:"estimate_minutes"`
	ActualMinutes   int64    `json:"actual_minutes"`
	Tags            []string `json:"tags"`
}

type timeInsights struct {
	Available        bool            `json:"available"`
	Samples          int             `json:"samples"`
	GlobalMultiplier float64         `json:"global_multiplier"`
	Tags             []tagMultiplier `json:"tags"`
	Recent           []recentSample  `json:"recent"`
}

// recentSampleCap bounds the per-task list returned for the insights screen.
const recentSampleCap = 60

func (h *taskHandler) timeInsights(w http.ResponseWriter, r *http.Request) {
	samples, err := h.store.CompletedTimeSamples(r.Context(), 1000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load time samples")
		return
	}
	ins := computeTimeInsights(samples)
	// Newest-first; attach a capped slice for the concrete recent-tasks list.
	recent := make([]recentSample, 0, recentSampleCap)
	for _, s := range samples {
		if len(recent) >= recentSampleCap {
			break
		}
		tags := s.Tags
		if tags == nil {
			tags = []string{}
		}
		recent = append(recent, recentSample{
			Title:           s.Title,
			EstimateMinutes: s.Estimate,
			ActualMinutes:   s.Actual,
			Tags:            tags,
		})
	}
	ins.Recent = recent
	respond(w, http.StatusOK, ins)
}

// computeTimeInsights derives multipliers from completed-task samples. We use
// the MEDIAN of per-task ratios (actual/estimate) rather than the mean so a
// single runaway task doesn't dominate the profile.
func computeTimeInsights(samples []db.TimeSample) timeInsights {
	global := make([]float64, 0, len(samples))
	byTag := map[string][]float64{}
	for _, s := range samples {
		if s.Estimate <= 0 {
			continue
		}
		ratio := float64(s.Actual) / float64(s.Estimate)
		global = append(global, ratio)
		for _, tag := range s.Tags {
			byTag[tag] = append(byTag[tag], ratio)
		}
	}

	out := timeInsights{Samples: len(global), Tags: []tagMultiplier{}}
	if len(global) >= minTimeSamples {
		out.Available = true
		out.GlobalMultiplier = round1(median(global))
	}
	for tag, ratios := range byTag {
		if len(ratios) < minTimeSamples {
			continue
		}
		out.Tags = append(out.Tags, tagMultiplier{
			Tag:        tag,
			Samples:    len(ratios),
			Multiplier: round1(median(ratios)),
		})
	}
	// Most-evidenced tags first; stable tie-break on name for deterministic output.
	sort.Slice(out.Tags, func(i, j int) bool {
		if out.Tags[i].Samples != out.Tags[j].Samples {
			return out.Tags[i].Samples > out.Tags[j].Samples
		}
		return out.Tags[i].Tag < out.Tags[j].Tag
	})
	return out
}

// globalTimeMultiplier returns the overall actual/estimate median, or 0 when
// there isn't enough data. Used to calibrate AI plan-day estimates.
func globalTimeMultiplier(ctx context.Context, store *db.TaskStore) float64 {
	samples, err := store.CompletedTimeSamples(ctx, 1000)
	if err != nil {
		return 0
	}
	ins := computeTimeInsights(samples)
	if !ins.Available {
		return 0
	}
	return ins.GlobalMultiplier
}

// pickSimilarSamples returns up to max past tasks most relevant to the given
// tags — tag matches first (samples are already newest-first), padded with
// recent tasks when there aren't enough tag matches to learn from.
func pickSimilarSamples(samples []db.TimeSample, tags []string, max int) []db.TimeSample {
	tagSet := map[string]bool{}
	for _, t := range tags {
		tagSet[t] = true
	}
	var matched, rest []db.TimeSample
	for _, s := range samples {
		hit := false
		for _, t := range s.Tags {
			if tagSet[t] {
				hit = true
				break
			}
		}
		if hit {
			matched = append(matched, s)
		} else {
			rest = append(rest, s)
		}
	}
	out := matched
	for _, s := range rest {
		if len(out) >= 3 {
			break
		}
		out = append(out, s)
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func round1(x float64) float64 {
	return float64(int(x*10+0.5)) / 10
}
