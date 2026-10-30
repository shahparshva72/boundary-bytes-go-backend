package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"
)

// keyPrefix is bumped when the cached entry format changes.
const keyPrefix = "bb:resp:v1"

// statExplorerRunPath is the one POST that is a pure read and worth caching:
// it is the heaviest endpoint and the body fully describes the result.
const statExplorerRunPath = "/api/stats/stat-explorer/run"

// uncachedPrefixes are per-client or live: rate-limit status, AI answers and
// feedback, and the external news feed.
var uncachedPrefixes = []string{
	"/api/text-to-sql",
	"/api/ai/",
	"/api/news",
}

// storeTimeout bounds a background cache write.
const storeTimeout = 5 * time.Second

// maxBodyBytes caps the stat-explorer request body read for hashing.
const maxBodyBytes = 64 << 10

type Versioner interface {
	Current(ctx context.Context) string
}

// Middleware serves cacheable requests from store and stores successful
// responses. Responses carry X-Cache: HIT or MISS.
func Middleware(store Store, version Versioner, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cacheable(r) {
				next.ServeHTTP(w, r)
				return
			}

			key, ok := requestKey(r, version.Current(r.Context()))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if entry, hit := store.Get(r.Context(), key); hit {
				if contentType, body, valid := decodeEntry(entry); valid {
					w.Header().Set("Content-Type", contentType)
					w.Header().Set("X-Cache", "HIT")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(body)
					return
				}
			}

			rec := &recorder{ResponseWriter: w, status: http.StatusOK}
			w.Header().Set("X-Cache", "MISS")
			next.ServeHTTP(rec, r)

			if rec.status == http.StatusOK && !rec.tooLarge {
				// Store in the background: the request context ends with the
				// handler, and the client shouldn't wait for Redis.
				entry := encodeEntry(w.Header().Get("Content-Type"), rec.body.Bytes())
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
					defer cancel()
					store.Set(ctx, key, entry, ttl)
				}()
			}
		})
	}
}

func cacheable(r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	for _, prefix := range uncachedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}

	switch r.Method {
	case http.MethodGet:
		// Without a seed the matchup game picks a random round.
		if path == "/api/games/matchup-round" && r.URL.Query().Get("seed") == "" {
			return false
		}
		return true
	case http.MethodPost:
		return path == statExplorerRunPath
	default:
		return false
	}
}

// requestKey builds the cache key. url.Values.Encode sorts parameters, so
// parameter order doesn't split the cache. POST bodies are hashed and put
// back for the handler.
func requestKey(r *http.Request, version string) (string, bool) {
	var b strings.Builder
	b.WriteString(keyPrefix)
	b.WriteString(":")
	b.WriteString(version)
	b.WriteString(":")
	b.WriteString(r.Method)
	b.WriteString(":")
	b.WriteString(r.URL.Path)
	b.WriteString("?")
	b.WriteString(r.URL.Query().Encode())

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
		if err != nil || len(body) > maxBodyBytes {
			return "", false
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := sha256.Sum256(body)
		b.WriteString("#")
		b.WriteString(hex.EncodeToString(sum[:]))
	}

	return b.String(), true
}

// An entry is the content type, a newline, then the body.
func encodeEntry(contentType string, body []byte) []byte {
	entry := make([]byte, 0, len(contentType)+1+len(body))
	entry = append(entry, contentType...)
	entry = append(entry, '\n')
	return append(entry, body...)
}

func decodeEntry(entry []byte) (string, []byte, bool) {
	i := bytes.IndexByte(entry, '\n')
	if i < 0 {
		return "", nil, false
	}
	return string(entry[:i]), entry[i+1:], true
}

// MaxEntryBytes is the largest response stored; bigger ones (e.g. every
// match across all leagues) are served uncached.
const MaxEntryBytes = 900 << 10

type recorder struct {
	http.ResponseWriter
	status   int
	body     bytes.Buffer
	tooLarge bool
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(p []byte) (int, error) {
	if !r.tooLarge {
		if r.body.Len()+len(p) > MaxEntryBytes {
			r.tooLarge = true
			r.body.Reset()
		} else {
			r.body.Write(p)
		}
	}
	return r.ResponseWriter.Write(p)
}
