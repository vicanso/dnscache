package dnscache

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

var defaultDialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

const defaultLookupTimeout = 10 * time.Second

// Cache is a concurrency-safe DNS cache with stale-while-revalidate and a dial helper.
type Cache struct {
	storage          Storage
	ttl              time.Duration
	stale            time.Duration
	negativeTTL      time.Duration
	policy           Policy
	network          string
	lookupTimeout    time.Duration
	dialTimeout      time.Duration
	resolver         IPResolver
	dialer           *net.Dialer
	onEvent          func(Event)
	fallbackToSystem bool
	rotate           bool

	sf        singleflight.Group
	hostIndex hostCounters

	// lifeMu serializes Close with background refresh scheduling.
	lifeMu sync.Mutex
	closed atomic.Bool
	wg     sync.WaitGroup
}

// New creates a Cache from cfg. Returns ErrInvalidConfig for bad values.
func New(cfg Config) (*Cache, error) {
	network := cfg.Network
	if network == "" {
		network = "ip"
	}
	switch network {
	case "ip", "ip4", "ip6":
	default:
		return nil, fmt.Errorf("%w: network must be ip, ip4, or ip6", ErrInvalidConfig)
	}
	if cfg.TTL < 0 || cfg.Stale < 0 || cfg.NegativeTTL < 0 || cfg.DialTimeout < 0 {
		return nil, fmt.Errorf("%w: durations must be non-negative", ErrInvalidConfig)
	}
	if cfg.Policy < 0 || cfg.Policy > PolicyRoundRobin {
		return nil, fmt.Errorf("%w: unknown policy", ErrInvalidConfig)
	}

	lookupTimeout := cfg.LookupTimeout
	if lookupTimeout == 0 {
		lookupTimeout = defaultLookupTimeout
	}
	if lookupTimeout < 0 {
		return nil, fmt.Errorf("%w: LookupTimeout must be non-negative", ErrInvalidConfig)
	}

	policy := cfg.Policy
	if policy == 0 {
		policy = PolicyInterleave
	}

	storage := cfg.Storage
	if storage == nil {
		storage = NewMemoryStorage(cfg.MaxEntries)
	}

	dialer := cfg.Dialer
	if dialer == nil {
		dialer = defaultDialer
	}

	resolver := cfg.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	return &Cache{
		storage:          storage,
		ttl:              cfg.TTL,
		stale:            cfg.Stale,
		negativeTTL:      cfg.NegativeTTL,
		policy:           policy,
		network:          network,
		lookupTimeout:    lookupTimeout,
		dialTimeout:      cfg.DialTimeout,
		resolver:         resolver,
		dialer:           dialer,
		onEvent:          cfg.OnEvent,
		fallbackToSystem: cfg.FallbackToSystem,
		rotate:           cfg.Rotate,
	}, nil
}

// Close marks the cache closed and waits for background refreshes to finish.
// In-flight Lookup/Dial calls may still complete; new background work is skipped.
func (c *Cache) Close() {
	c.lifeMu.Lock()
	if c.closed.Swap(true) {
		c.lifeMu.Unlock()
		return
	}
	c.lifeMu.Unlock()
	c.wg.Wait()
}

// LookupHost returns addresses for host, using the cache when possible.
func (c *Cache) LookupHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	host = normalizeHost(host)
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrNotFound)
	}
	if addr, ok := parseIPLiteral(host); ok {
		return []netip.Addr{addr}, nil
	}

	now := time.Now()
	if entry, err := c.storage.Get(ctx, host); err == nil {
		if entry.Negative && entry.IsFresh(now) {
			c.emit(Event{Kind: EventNegative, Host: host})
			return nil, ErrNegativeCache
		}
		if !entry.Negative && entry.IsFresh(now) {
			c.emit(Event{Kind: EventCacheHit, Host: host, Addrs: cloneAddrs(entry.Addrs)})
			return cloneAddrs(entry.Addrs), nil
		}
		if !entry.Negative && entry.IsStale(now) {
			c.emit(Event{Kind: EventCacheStale, Host: host, Addrs: cloneAddrs(entry.Addrs)})
			c.refreshAsync(host)
			return cloneAddrs(entry.Addrs), nil
		}
	}

	return c.lookupSingleflight(ctx, host, false)
}

// LookupIP is like LookupHost but returns []net.IP for older call sites.
func (c *Cache) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := c.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	return ipToNetIP(addrs), nil
}

// Set stores an entry under host (normalized). Use Permanent() for never-expire.
func (c *Cache) Set(ctx context.Context, host string, entry Entry) error {
	if c.closed.Load() {
		return ErrClosed
	}
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrInvalidConfig)
	}
	entry.Addrs = dedupeAddrs(entry.Addrs)
	return c.storage.Set(ctx, host, entry)
}

// Get returns the raw cache entry for host.
func (c *Cache) Get(ctx context.Context, host string) (Entry, error) {
	if c.closed.Load() {
		return Entry{}, ErrClosed
	}
	return c.storage.Get(ctx, normalizeHost(host))
}

// Delete removes the cache entry for host.
func (c *Cache) Delete(ctx context.Context, host string) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.storage.Delete(ctx, normalizeHost(host))
}

// Purge drops all entries when Storage implements Purger.
func (c *Cache) Purge(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClosed
	}
	p, ok := c.storage.(Purger)
	if !ok {
		return fmt.Errorf("%w: storage does not support Purge", ErrInvalidConfig)
	}
	return p.Purge(ctx)
}

// Warm resolves and caches each host. Returns a joined error for failures.
func (c *Cache) Warm(ctx context.Context, hosts ...string) error {
	if c.closed.Load() {
		return ErrClosed
	}
	var errs []error
	for _, h := range hosts {
		name := normalizeHost(h)
		if name == "" {
			errs = append(errs, fmt.Errorf("%s: empty host", h))
			continue
		}
		if _, ok := parseIPLiteral(name); ok {
			continue
		}
		if _, err := c.lookupSingleflight(ctx, name, false); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", h, err))
		}
	}
	return errors.Join(errs...)
}

func (c *Cache) lookupSingleflight(ctx context.Context, host string, isRefresh bool) ([]netip.Addr, error) {
	type result struct {
		addrs []netip.Addr
	}
	v, err, _ := c.sf.Do(host, func() (any, error) {
		addrs, err := c.lookupAndStore(ctx, host, isRefresh)
		if err != nil {
			return nil, err
		}
		return result{addrs: addrs}, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneAddrs(v.(result).addrs), nil
}

func (c *Cache) lookupAndStore(ctx context.Context, host string, isRefresh bool) ([]netip.Addr, error) {
	start := time.Now()
	addrs, err := c.lookup(ctx, host)
	if err != nil {
		if c.negativeTTL > 0 {
			now := time.Now()
			_ = c.storage.Set(ctx, host, Entry{
				Negative:   true,
				FreshUntil: now.Add(c.negativeTTL),
			})
			c.emit(Event{Kind: EventNegative, Host: host, Duration: time.Since(start), Err: err})
		}
		if !isRefresh {
			c.emit(Event{Kind: EventError, Host: host, Duration: time.Since(start), Err: err})
		}
		return nil, err
	}

	now := time.Now()
	entry := Entry{Addrs: addrs}
	if c.ttl > 0 {
		entry.FreshUntil = now.Add(c.ttl)
		if c.stale > 0 {
			entry.StaleUntil = entry.FreshUntil.Add(c.stale)
		}
		if setErr := c.storage.Set(ctx, host, entry); setErr != nil {
			return nil, setErr
		}
	}

	kind := EventLookup
	if isRefresh {
		kind = EventRefresh
	}
	c.emit(Event{Kind: kind, Host: host, Duration: time.Since(start), Addrs: cloneAddrs(addrs)})
	return addrs, nil
}

func (c *Cache) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	timeout := c.lookupTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ips, err := c.resolver.LookupIP(ctx, c.network, host)
	if err != nil {
		return nil, err
	}
	addrs := netIPToAddr(ips)
	if len(addrs) == 0 {
		return nil, ErrNotFound
	}
	return addrs, nil
}

func (c *Cache) refreshAsync(host string) {
	c.lifeMu.Lock()
	if c.closed.Load() {
		c.lifeMu.Unlock()
		return
	}
	c.wg.Add(1)
	c.lifeMu.Unlock()

	go func() {
		defer c.wg.Done()
		if c.closed.Load() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), c.lookupTimeout)
		defer cancel()
		_, _ = c.lookupSingleflight(ctx, host, true)
	}()
}

func (c *Cache) emit(ev Event) {
	if c.onEvent == nil {
		return
	}
	c.onEvent(ev)
}
