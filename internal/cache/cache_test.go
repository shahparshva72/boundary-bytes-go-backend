package cache

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreEvictsLeastRecentlyUsed(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(10)
	m.Set(ctx, "a", []byte("aaaa"), time.Minute)
	m.Set(ctx, "b", []byte("bbbb"), time.Minute)
	m.Get(ctx, "a")                              // a is now most recently used
	m.Set(ctx, "c", []byte("cccc"), time.Minute) // over 10 bytes: evicts b

	if _, ok := m.Get(ctx, "b"); ok {
		t.Error("b should have been evicted")
	}
	for _, k := range []string{"a", "c"} {
		if _, ok := m.Get(ctx, k); !ok {
			t.Errorf("%s should still be cached", k)
		}
	}

	m.Set(ctx, "huge", []byte(strings.Repeat("x", 11)), time.Minute)
	if _, ok := m.Get(ctx, "huge"); ok {
		t.Error("an entry larger than the store must not be cached")
	}
}

func TestMemoryStoreExpires(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore(100)
	now := time.Now()
	m.now = func() time.Time { return now }
	m.Set(ctx, "k", []byte("v"), time.Minute)

	now = now.Add(2 * time.Minute)
	if _, ok := m.Get(ctx, "k"); ok {
		t.Error("expired entry returned")
	}
	if m.size != 0 {
		t.Errorf("expired entry still counted: size=%d", m.size)
	}
}

func TestTieredStoreFillsLocalFromShared(t *testing.T) {
	ctx := context.Background()
	local, shared := NewMemoryStore(100), NewMemoryStore(100)
	shared.Set(ctx, "k", []byte("v"), time.Hour)

	tiered := NewTieredStore(local, shared, time.Minute)
	if v, ok := tiered.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("tiered Get = %q, %v", v, ok)
	}
	if _, ok := local.Get(ctx, "k"); !ok {
		t.Error("local store not filled on shared hit")
	}
}

// recordingStore wraps a MemoryStore and lets tests wait for async writes.
type recordingStore struct {
	*MemoryStore
	mu   sync.Mutex
	sets int
}

func (s *recordingStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	s.MemoryStore.Set(ctx, key, value, ttl)
	s.mu.Lock()
	s.sets++
	s.mu.Unlock()
}

func (s *recordingStore) waitForSets(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := s.sets
		s.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d cache writes", n)
}

type fixedVersion struct{ v string }

func (f *fixedVersion) Current(context.Context) string { return f.v }

func newTestServer(t *testing.T, version *fixedVersion) (*httptest.Server, *recordingStore, *int) {
	t.Helper()
	store := &recordingStore{MemoryStore: NewMemoryStore(1 << 20)}
	calls := 0
	var mu sync.Mutex

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		switch r.URL.Path {
		case "/api/fail":
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		case "/api/large":
			w.Write([]byte(strings.Repeat("x", MaxEntryBytes+1)))
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"path":"` + r.URL.Path + `","query":"` + r.URL.RawQuery + `","body":"` + string(body) + `"}`))
	})

	srv := httptest.NewServer(Middleware(store, version, time.Hour)(handler))
	t.Cleanup(srv.Close)
	return srv, store, &calls
}

func do(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestMiddlewareCachesGetAndIgnoresParamOrder(t *testing.T) {
	srv, store, calls := newTestServer(t, &fixedVersion{"v1"})

	resp, first := do(t, http.MethodGet, srv.URL+"/api/stats/leading-run-scorers?league=IPL&limit=5", "")
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Fatalf("first request X-Cache = %q", resp.Header.Get("X-Cache"))
	}
	store.waitForSets(t, 1)

	resp, second := do(t, http.MethodGet, srv.URL+"/api/stats/leading-run-scorers?limit=5&league=IPL", "")
	if resp.Header.Get("X-Cache") != "HIT" || second != first || *calls != 1 {
		t.Fatalf("reordered params should hit: X-Cache=%q calls=%d", resp.Header.Get("X-Cache"), *calls)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content type not restored: %q", resp.Header.Get("Content-Type"))
	}
}

func TestMiddlewareNewDataVersionMisses(t *testing.T) {
	version := &fixedVersion{"v1"}
	srv, store, calls := newTestServer(t, version)

	do(t, http.MethodGet, srv.URL+"/api/stats/team-wins?league=IPL", "")
	store.waitForSets(t, 1)

	version.v = "v2" // the parser loaded new matches
	resp, _ := do(t, http.MethodGet, srv.URL+"/api/stats/team-wins?league=IPL", "")
	if resp.Header.Get("X-Cache") != "MISS" || *calls != 2 {
		t.Errorf("new data version should miss: X-Cache=%q calls=%d", resp.Header.Get("X-Cache"), *calls)
	}
}

func TestMiddlewareStatExplorerPostKeyedByBody(t *testing.T) {
	srv, store, calls := newTestServer(t, &fixedVersion{"v1"})
	url := srv.URL + statExplorerRunPath + "?league=IPL"

	_, a := do(t, http.MethodPost, url, `{"reportType":"batting"}`)
	if !strings.Contains(a, `"body":"{"reportType":"batting"}"`) {
		t.Fatalf("handler did not receive the request body: %s", a)
	}
	store.waitForSets(t, 1)

	resp, _ := do(t, http.MethodPost, url, `{"reportType":"batting"}`)
	if resp.Header.Get("X-Cache") != "HIT" {
		t.Errorf("same body should hit, got %q", resp.Header.Get("X-Cache"))
	}
	resp, _ = do(t, http.MethodPost, url, `{"reportType":"bowling"}`)
	if resp.Header.Get("X-Cache") != "MISS" || *calls != 2 {
		t.Errorf("different body should miss: X-Cache=%q calls=%d", resp.Header.Get("X-Cache"), *calls)
	}
}

func TestMiddlewareSkipsUncacheableRequests(t *testing.T) {
	srv, _, calls := newTestServer(t, &fixedVersion{"v1"})

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/text-to-sql/limits"},
		{http.MethodPost, "/api/text-to-sql"},
		{http.MethodGet, "/api/ai/feedback"},
		{http.MethodPost, "/api/ai/feedback"},
		{http.MethodGet, "/api/news"},
		{http.MethodGet, "/api/games/matchup-round?league=IPL"},
		{http.MethodGet, "/health"},
	} {
		for range 2 {
			resp, _ := do(t, tc.method, srv.URL+tc.path, "")
			if xc := resp.Header.Get("X-Cache"); xc != "" {
				t.Errorf("%s %s should bypass the cache, got X-Cache=%q", tc.method, tc.path, xc)
			}
		}
	}
	if *calls != 14 {
		t.Errorf("every uncacheable request should reach the handler: calls=%d", *calls)
	}

	resp, _ := do(t, http.MethodGet, srv.URL+"/api/games/matchup-round?league=IPL&seed=abc", "")
	if resp.Header.Get("X-Cache") != "MISS" {
		t.Error("a seeded matchup round is deterministic and should be cached")
	}
}

func TestMiddlewareDoesNotStoreErrorsOrLargeResponses(t *testing.T) {
	srv, store, _ := newTestServer(t, &fixedVersion{"v1"})

	do(t, http.MethodGet, srv.URL+"/api/fail", "")
	_, body := do(t, http.MethodGet, srv.URL+"/api/large", "")
	if len(body) != MaxEntryBytes+1 {
		t.Fatalf("large response truncated: %d bytes", len(body))
	}
	time.Sleep(50 * time.Millisecond)

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.sets != 0 {
		t.Errorf("errors and oversized responses must not be stored: %d writes", store.sets)
	}
}
