package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The account credential must never be sent off the configured server, even
// when a stored calendar href is an absolute URL to another host.
func TestCredentialStaysOnConfiguredOrigin(t *testing.T) {
	var gotAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(201)
	}))
	defer other.Close()
	c, err := NewClient(Config{BaseURL: FastmailBaseURL, Username: "u@example.test", Password: "APP-PW"})
	if err != nil {
		t.Fatal(err)
	}
	if c.SameOrigin(other.URL + "/cal/") {
		t.Fatal("foreign href reported as same origin")
	}
	if !c.SameOrigin("/dav/calendars/user/u@example.test/abc/") {
		t.Fatal("server path rejected")
	}
	err = c.PutEvent(context.Background(), other.URL+"/cal/", "uid-1", "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("PutEvent to another origin: err=%v", err)
	}
	if gotAuth != "" {
		t.Fatalf("credential sent to another host: %q", gotAuth)
	}
}
