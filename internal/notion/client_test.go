package notion_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/yourorg/notionctl/internal/notion"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*notion.Client, func()) {
	t.Helper()

	server := httptest.NewServer(handler)

	cfg := notion.ClientConfig{
		Token:   "test-token",
		BaseURL: server.URL + "/",
	}
	client := notion.NewClient(cfg)
	client.WithLimiter(rate.NewLimiter(rate.Inf, 0))
	client.WithSleeper(func(time.Duration) {})

	return client, server.Close
}

func TestClientSetsHeaders(t *testing.T) {
	var capturedHeaders http.Header

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer cleanup()

	if err := client.Do(context.Background(), "GET", "/ping", nil, &struct{ OK bool }{}); err != nil {
		t.Fatalf("callDo returned error: %v", err)
	}

	if got, want := capturedHeaders.Get("Authorization"), "Bearer test-token"; got != want {
		t.Fatalf("Authorization header = %q, want %q", got, want)
	}
	if got := capturedHeaders.Get("Notion-Version"); got == "" {
		t.Fatalf("Notion-Version header missing")
	}
}

func TestClientRetriesOn429(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"status":429,"code":"rate_limited","message":"slow down"}`)); err != nil {
				t.Fatalf("write retry response: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("write success response: %v", err)
		}
	})
	defer cleanup()

	var waitCalls int
	client.WithSleeper(func(d time.Duration) {
		waitCalls++
	})

	if err := client.Do(context.Background(), "GET", "/ping", nil, &struct{ OK bool }{}); err != nil {
		t.Fatalf("callDo returned error: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if waitCalls == 0 {
		t.Fatalf("expected sleep to be invoked for retry")
	}
}

func TestClientRetriesOn5xx(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte(`{"status":503,"code":"unavailable","message":"try again"}`)); err != nil {
				t.Fatalf("write retry response: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("write success response: %v", err)
		}
	})
	defer cleanup()

	if err := client.Do(context.Background(), "GET", "/ping", nil, &struct{ OK bool }{}); err != nil {
		t.Fatalf("callDo returned error: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestListDataSources(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/databases/db123/data_sources" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"results": []map[string]any{
				{"id": "ds1", "name": "Main"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	})
	defer cleanup()

	dataSources, err := client.ListDataSources(context.Background(), "db123")
	if err != nil {
		t.Fatalf("ListDataSources returned error: %v", err)
	}
	if len(dataSources) != 1 || dataSources[0].ID != "ds1" {
		t.Fatalf("unexpected data sources: %#v", dataSources)
	}
}
