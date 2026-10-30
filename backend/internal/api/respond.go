package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"time"

	"github.com/google/uuid"
)

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(emptyIfNilSlice(v))
	}
}

// emptyIfNilSlice turns a nil slice into an empty one so it encodes as [] rather
// than null. Several stores return a nil slice for "no rows", and a client doing
// `rows.length` / `rows.filter` on null crashes — on web that took down the whole
// task panel (SubTaskList on a task with no sub-tasks).
func emptyIfNilSlice(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	// Limit request bodies to 1 MB to prevent memory exhaustion
	limited := http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(limited).Decode(v)
}

func mondayOfDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return t.AddDate(0, 0, -(wd - 1)).Format("2006-01-02")
}

func newID() string {
	return uuid.New().String()
}

// clientOrNewID returns the client-supplied id when it is a valid UUID, else a
// fresh one. Offline clients generate the id locally so it survives sync (the
// local row and the server row share one id, keeping foreign keys stable).
func clientOrNewID(id string) string {
	if id != "" {
		if _, err := uuid.Parse(id); err == nil {
			return id
		}
	}
	return newID()
}
