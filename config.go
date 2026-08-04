package dnscache

import (
	"context"
	"net"
	"net/netip"
	"time"
)

// Policy selects how resolved addresses are ordered for dialing.
// The zero value means “use the library default” (PolicyInterleave).
type Policy int

const (
	// PolicyInterleave interleaves address families (Happy Eyeballs style)
	// while preserving the resolver's preferred lead family. This is the default.
	PolicyInterleave Policy = iota + 1
	// PolicyFirst keeps resolver order and dials the first usable address first.
	PolicyFirst
	// PolicyRandom shuffles addresses before dialing.
	PolicyRandom
	// PolicyRoundRobin rotates the starting address per host on each dial.
	PolicyRoundRobin
)

// EventKind classifies cache/dial lifecycle events.
type EventKind int

const (
	// EventCacheHit is emitted when a fresh cache entry is returned.
	EventCacheHit EventKind = iota
	// EventCacheStale is emitted when a stale entry is served while a refresh runs.
	EventCacheStale
	// EventLookup is emitted after a successful upstream DNS lookup.
	EventLookup
	// EventRefresh is emitted after a background stale refresh finishes.
	EventRefresh
	// EventNegative is emitted when a negative cache entry is stored or hit.
	EventNegative
	// EventDial is emitted for each dial attempt (success or failure).
	EventDial
	// EventError is emitted for lookup/dial failures that escape to the caller.
	EventError
)

// Event is a structured observability hook payload.
type Event struct {
	Kind     EventKind
	Host     string
	Duration time.Duration
	Addrs    []netip.Addr
	Network  string
	Address  string
	Err      error
}

// IPResolver performs host lookups. *net.Resolver implements this interface.
type IPResolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// Config configures a Cache. Zero values get sensible defaults in New.
type Config struct {
	// TTL is how long a successful lookup stays fresh.
	// If zero, successful lookups are not cached (manual Set entries still work).
	TTL time.Duration

	// Stale is the stale-while-revalidate window after TTL.
	// During this window the cache returns the old entry and refreshes in the background.
	Stale time.Duration

	// NegativeTTL caches lookup failures for this duration. Zero disables negative caching.
	NegativeTTL time.Duration

	// MaxEntries bounds the default in-memory storage. Zero defaults to 4096.
	// Ignored when a custom Storage is provided (unless that storage uses it).
	MaxEntries int

	// Policy controls dial address ordering. Zero defaults to PolicyInterleave.
	Policy Policy

	// Network limits lookups: "ip" (default), "ip4", or "ip6".
	Network string

	// LookupTimeout bounds each upstream DNS lookup. Default: 10s.
	LookupTimeout time.Duration

	// DialTimeout bounds each individual dial attempt. Zero means no per-attempt
	// timeout beyond the caller's context and Dialer.Timeout.
	DialTimeout time.Duration

	// Resolver is used for upstream lookups. Nil uses net.DefaultResolver.
	// *net.Resolver or any IPResolver implementation is accepted.
	Resolver IPResolver

	// Dialer is used for DialContext. Nil uses a default dialer (30s timeout/keepalive).
	Dialer *net.Dialer

	// Storage overrides the default bounded in-memory cache.
	Storage Storage

	// OnEvent, if set, is called synchronously for cache/dial events.
	// Handlers must be fast and non-blocking.
	OnEvent func(Event)

	// FallbackToSystem makes DialContext fall back to dialing the original
	// host:port via the system resolver when the cache has no usable address.
	FallbackToSystem bool

	// Rotate advances the starting offset within each address family when
	// using PolicyInterleave. RoundRobin always rotates per host.
	Rotate bool
}
