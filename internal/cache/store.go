// Package cache caches API responses. Stats only change when the parser
// loads data, and the default all-league pages are identical for every
// visitor, so most requests can skip the database.
package cache

import (
	"container/list"
	"context"
	"log"
	"sync"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/ratelimit"
)

// Store is a byte cache. Implementations fail open: a broken store behaves
// like an empty one, so the API keeps serving from the database.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
}

// MemoryStore is a per-instance LRU bounded by total bytes, so it can't
// exhaust a small Cloud Run instance.
type MemoryStore struct {
	mu       sync.Mutex
	maxBytes int
	size     int
	order    *list.List // front = most recently used
	entries  map[string]*list.Element
	now      func() time.Time
}

type memoryEntry struct {
	key       string
	value     []byte
	expiresAt time.Time
}

func NewMemoryStore(maxBytes int) *MemoryStore {
	return &MemoryStore{
		maxBytes: maxBytes,
		order:    list.New(),
		entries:  make(map[string]*list.Element),
		now:      time.Now,
	}
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	el, ok := m.entries[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*memoryEntry)
	if m.now().After(entry.expiresAt) {
		m.remove(el)
		return nil, false
	}
	m.order.MoveToFront(el)
	return entry.value, true
}

func (m *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) {
	if len(value) > m.maxBytes {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if el, ok := m.entries[key]; ok {
		m.remove(el)
	}
	el := m.order.PushFront(&memoryEntry{key: key, value: value, expiresAt: m.now().Add(ttl)})
	m.entries[key] = el
	m.size += len(value)

	for m.size > m.maxBytes {
		m.remove(m.order.Back())
	}
}

func (m *MemoryStore) remove(el *list.Element) {
	entry := el.Value.(*memoryEntry)
	m.order.Remove(el)
	delete(m.entries, entry.key)
	m.size -= len(entry.value)
}

// RedisStore keeps entries in Upstash, shared by every instance and kept
// across cold starts.
type RedisStore struct {
	client   *ratelimit.UpstashClient
	maxBytes int
}

func NewRedisStore(client *ratelimit.UpstashClient, maxBytes int) *RedisStore {
	return &RedisStore{client: client, maxBytes: maxBytes}
}

func (r *RedisStore) Get(ctx context.Context, key string) ([]byte, bool) {
	// JSON-escaping the stored string can roughly double it in the reply.
	value, ok, err := r.client.GetString(ctx, key, int64(r.maxBytes)*2+1024)
	if err != nil {
		log.Printf("cache: redis get failed, serving from database: %v", err)
		return nil, false
	}
	return []byte(value), ok
}

func (r *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if len(value) > r.maxBytes {
		return
	}
	if err := r.client.SetEx(ctx, key, string(value), int(ttl.Seconds())); err != nil {
		log.Printf("cache: redis set failed: %v", err)
	}
}

// TieredStore reads the fast local store first and fills it from the
// shared one.
type TieredStore struct {
	local, shared Store
	localTTL      time.Duration
}

func NewTieredStore(local, shared Store, localTTL time.Duration) *TieredStore {
	return &TieredStore{local: local, shared: shared, localTTL: localTTL}
}

func (t *TieredStore) Get(ctx context.Context, key string) ([]byte, bool) {
	if value, ok := t.local.Get(ctx, key); ok {
		return value, true
	}
	value, ok := t.shared.Get(ctx, key)
	if ok {
		t.local.Set(ctx, key, value, t.localTTL)
	}
	return value, ok
}

func (t *TieredStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	t.local.Set(ctx, key, value, min(ttl, t.localTTL))
	t.shared.Set(ctx, key, value, ttl)
}
