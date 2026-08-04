package dnscache

import "context"

// Storage is the cache backend seam. Implementations must be safe for concurrent use.
//
// Get returns ErrNotFound when the host is absent.
// Set replaces any existing entry for host.
type Storage interface {
	Get(ctx context.Context, host string) (Entry, error)
	Set(ctx context.Context, host string, entry Entry) error
	Delete(ctx context.Context, host string) error
}

// Purger is an optional Storage extension for dropping all entries.
type Purger interface {
	Purge(ctx context.Context) error
}
