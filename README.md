# DNS Cache

[![Test](https://github.com/vicanso/dnscache/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/vicanso/dnscache/actions/workflows/test.yml)

Process-local DNS cache for Go HTTP clients and dialers.

Go does not cache DNS lookups. This package provides:

- TTL cache with **stale-while-revalidate**
- **singleflight** coalescing of concurrent misses and background refreshes
- Optional **negative caching**
- Bounded in-memory LRU storage (pluggable `Storage`)
- `DialContext` that tries multiple addresses (family interleave, round-robin, random)
- Host normalization (ASCII case fold, trailing-dot strip)

## Install

```bash
go get github.com/vicanso/dnscache
```

## Quick start

```go
package main

import (
	"net/http"
	"time"

	"github.com/vicanso/dnscache"
)

func main() {
	c, err := dnscache.New(dnscache.Config{
		TTL:   time.Minute,
		Stale: 30 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: c.DialContext,
		},
	}
	resp, err := client.Get("https://example.com/")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
```

## Config

| Field | Default | Meaning |
| --- | --- | --- |
| `TTL` | `0` (no auto-cache) | Fresh lifetime of successful lookups |
| `Stale` | `0` | Extra window to serve stale data while refreshing |
| `NegativeTTL` | `0` | Cache lookup failures for this long |
| `MaxEntries` | `4096` | LRU capacity for default memory storage |
| `Policy` | `PolicyInterleave` | Dial address ordering |
| `Network` | `"ip"` | Lookup network: `ip`, `ip4`, `ip6` |
| `LookupTimeout` | `10s` | Per-lookup timeout |
| `DialTimeout` | `0` | Optional per-attempt dial timeout |
| `Resolver` | `net.DefaultResolver` | `IPResolver` implementation |
| `Dialer` | 30s timeout/keepalive | Underlying dialer |
| `Storage` | memory LRU | Cache backend |
| `OnEvent` | `nil` | Observability hook |
| `FallbackToSystem` | `false` | Dial original host on cache failure |
| `Rotate` | `false` | Rotate within families for `PolicyInterleave` |

### Policies

- `PolicyInterleave` — alternate address families (Happy Eyeballs style ordering)
- `PolicyFirst` — resolver order
- `PolicyRandom` — shuffle
- `PolicyRoundRobin` — per-host rotating start index

## API

### LookupHost / LookupIP

```go
addrs, err := c.LookupHost(ctx, "example.com")
// or legacy []net.IP:
ips, err := c.LookupIP(ctx, "example.com")
```

IP literals (`127.0.0.1`, `::1`, `[::1]`) bypass the resolver and cache.

### Manual cache control

```go
// Never expires (FreshUntil/StaleUntil zero)
_ = c.Set(ctx, "api.internal", dnscache.Permanent(
	netip.MustParseAddr("192.0.2.10"),
))

entry, err := c.Get(ctx, "api.internal")
_ = c.Delete(ctx, "api.internal")
_ = c.Purge(ctx) // memory storage
_ = c.Warm(ctx, "a.example.com", "b.example.com")
```

### Events

```go
c, _ := dnscache.New(dnscache.Config{
	TTL: time.Minute,
	OnEvent: func(ev dnscache.Event) {
		// EventCacheHit, EventCacheStale, EventLookup, EventRefresh,
		// EventNegative, EventDial, EventError
	},
})
```

### Custom storage

```go
type Storage interface {
	Get(ctx context.Context, host string) (Entry, error)
	Set(ctx context.Context, host string, entry Entry) error
	Delete(ctx context.Context, host string) error
}
```

## Breaking changes (v2-style rewrite)

This package no longer exposes mutable fields on the cache instance. The previous
`New(ttl, opts...)`, `GetDialContext(...bool)`, `net.IP`-based `IPCache`, and
integer policy constants have been replaced by `Config`, `DialContext`,
`netip.Addr` entries, and typed `Policy` values.

## License

Apache-2.0
