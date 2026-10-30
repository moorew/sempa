package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clevercode/sempa/internal/db"
)

func TestRespondNilSliceIsEmptyArray(t *testing.T) {
	var tasks []db.Task
	rec := httptest.NewRecorder()
	respond(rec, 200, tasks)
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("nil slice encoded as %q, want []", got)
	}
	rec = httptest.NewRecorder()
	respond(rec, 200, map[string]string{"a": "b"})
	if got := strings.TrimSpace(rec.Body.String()); got != `{"a":"b"}` {
		t.Fatalf("map encoded as %q", got)
	}
}
