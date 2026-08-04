package dnscache

import (
	"container/list"
	"context"
	"sync"
)

const defaultMaxEntries = 4096

// MemoryStorage is a bounded in-memory LRU Storage.
type MemoryStorage struct {
	mu      sync.Mutex
	max     int
	items   map[string]*list.Element
	order   *list.List
}

type memItem struct {
	host  string
	entry Entry
}

// NewMemoryStorage creates an LRU storage with the given capacity.
// maxEntries <= 0 defaults to 4096.
func NewMemoryStorage(maxEntries int) *MemoryStorage {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &MemoryStorage{
		max:   maxEntries,
		items: make(map[string]*list.Element, maxEntries),
		order: list.New(),
	}
}

// Get implements Storage.
func (s *MemoryStorage) Get(_ context.Context, host string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[host]
	if !ok {
		return Entry{}, ErrNotFound
	}
	s.order.MoveToFront(el)
	return cloneEntry(el.Value.(*memItem).entry), nil
}

// Set implements Storage.
func (s *MemoryStorage) Set(_ context.Context, host string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry = cloneEntry(entry)
	if el, ok := s.items[host]; ok {
		el.Value.(*memItem).entry = entry
		s.order.MoveToFront(el)
		return nil
	}
	for s.order.Len() >= s.max {
		s.evictOldest()
	}
	el := s.order.PushFront(&memItem{host: host, entry: entry})
	s.items[host] = el
	return nil
}

// Delete implements Storage.
func (s *MemoryStorage) Delete(_ context.Context, host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[host]
	if !ok {
		return nil
	}
	s.order.Remove(el)
	delete(s.items, host)
	return nil
}

// Purge implements Purger.
func (s *MemoryStorage) Purge(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*list.Element, s.max)
	s.order.Init()
	return nil
}

// Len returns the number of cached entries.
func (s *MemoryStorage) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.order.Len()
}

func (s *MemoryStorage) evictOldest() {
	el := s.order.Back()
	if el == nil {
		return
	}
	item := el.Value.(*memItem)
	s.order.Remove(el)
	delete(s.items, item.host)
}
