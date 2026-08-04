// Package dnscache provides a concurrency-safe process-local DNS cache for
// Go HTTP clients and dialers.
//
// Go's standard library does not cache DNS lookups. High-churn clients that
// repeatedly connect to the same hosts pay resolver latency on every new
// connection. This package sits in that gap:
//
//   - Bounded in-memory cache with TTL and stale-while-revalidate
//   - singleflight coalescing of concurrent misses and background refreshes
//   - Optional negative caching of lookup failures
//   - DialContext that tries multiple addresses with family interleaving
//   - Pluggable Storage and IPResolver seams for tests and custom backends
//
// Basic usage:
//
//	c, err := dnscache.New(dnscache.Config{TTL: time.Minute, Stale: 30 * time.Second})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	client := &http.Client{
//	    Transport: &http.Transport{DialContext: c.DialContext},
//	}
package dnscache
