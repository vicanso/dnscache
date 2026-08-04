package dnscache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultsAndValidation(t *testing.T) {
	c, err := New(Config{})
	require.NoError(t, err)
	assert.Equal(t, PolicyInterleave, c.policy)
	assert.Equal(t, "ip", c.network)
	assert.Equal(t, defaultLookupTimeout, c.lookupTimeout)
	c.Close()

	_, err = New(Config{Network: "tcp"})
	assert.ErrorIs(t, err, ErrInvalidConfig)

	_, err = New(Config{TTL: -1})
	assert.ErrorIs(t, err, ErrInvalidConfig)

	c, err = New(Config{Policy: PolicyFirst})
	require.NoError(t, err)
	assert.Equal(t, PolicyFirst, c.policy)
	c.Close()
}

func TestLookupHostCacheHitAndNormalize(t *testing.T) {
	r := newFakeResolver()
	r.set("Example.COM", "192.0.2.1", "2001:db8::1")
	c, err := New(Config{
		TTL:      time.Minute,
		Resolver: r,
		Policy:   PolicyFirst,
	})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	addrs, err := c.LookupHost(ctx, "example.com.")
	require.NoError(t, err)
	require.Len(t, addrs, 2)
	assert.Equal(t, int64(1), r.lookups.Load())

	addrs2, err := c.LookupHost(ctx, "EXAMPLE.com")
	require.NoError(t, err)
	assert.Equal(t, addrs, addrs2)
	assert.Equal(t, int64(1), r.lookups.Load())
}

func TestLookupHostIPLiteral(t *testing.T) {
	r := newFakeResolver()
	c, err := New(Config{TTL: time.Minute, Resolver: r})
	require.NoError(t, err)
	defer c.Close()

	addrs, err := c.LookupHost(context.Background(), "127.0.0.1")
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "127.0.0.1", addrs[0].String())
	assert.Equal(t, int64(0), r.lookups.Load())

	addrs, err = c.LookupHost(context.Background(), "[::1]")
	require.NoError(t, err)
	assert.Equal(t, "::1", addrs[0].String())
}

func TestSingleflightCoalescesConcurrentMiss(t *testing.T) {
	r := newFakeResolver()
	r.set("sf.example", "192.0.2.10")
	r.delay = 50 * time.Millisecond
	c, err := New(Config{TTL: time.Minute, Resolver: r})
	require.NoError(t, err)
	defer c.Close()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := c.LookupHost(context.Background(), "sf.example")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), r.lookups.Load())
}

func TestStaleWhileRevalidate(t *testing.T) {
	r := newFakeResolver()
	r.set("stale.example", "192.0.2.1")
	var events []EventKind
	var mu sync.Mutex
	c, err := New(Config{
		TTL:      50 * time.Millisecond,
		Stale:    500 * time.Millisecond,
		Resolver: r,
		OnEvent: func(ev Event) {
			mu.Lock()
			events = append(events, ev.Kind)
			mu.Unlock()
		},
	})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	addrs, err := c.LookupHost(ctx, "stale.example")
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", addrs[0].String())
	assert.Equal(t, int64(1), r.lookups.Load())

	time.Sleep(80 * time.Millisecond)
	r.set("stale.example", "192.0.2.99")

	addrs2, err := c.LookupHost(ctx, "stale.example")
	require.NoError(t, err)
	// still serves stale
	assert.Equal(t, "192.0.2.1", addrs2[0].String())

	// wait for background refresh
	require.Eventually(t, func() bool {
		return r.lookups.Load() >= 2
	}, time.Second, 10*time.Millisecond)

	// after refresh, next hit should be fresh new value
	require.Eventually(t, func() bool {
		addrs3, err := c.LookupHost(ctx, "stale.example")
		return err == nil && len(addrs3) > 0 && addrs3[0].String() == "192.0.2.99"
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, events, EventCacheStale)
	assert.Contains(t, events, EventRefresh)
}

func TestNegativeCache(t *testing.T) {
	r := newFakeResolver()
	r.setErr(errors.New("servfail"))
	c, err := New(Config{
		TTL:         time.Minute,
		NegativeTTL: 200 * time.Millisecond,
		Resolver:    r,
	})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	_, err = c.LookupHost(ctx, "missing.example")
	require.Error(t, err)
	assert.Equal(t, int64(1), r.lookups.Load())

	_, err = c.LookupHost(ctx, "missing.example")
	assert.ErrorIs(t, err, ErrNegativeCache)
	assert.Equal(t, int64(1), r.lookups.Load())

	time.Sleep(250 * time.Millisecond)
	r.setErr(nil)
	r.set("missing.example", "192.0.2.5")
	addrs, err := c.LookupHost(ctx, "missing.example")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.5", addrs[0].String())
}

func TestSetGetDeletePermanent(t *testing.T) {
	c, err := New(Config{TTL: time.Minute, Resolver: newFakeResolver()})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "Manual.Example", Permanent(mustAddr("198.51.100.1"))))
	entry, err := c.Get(ctx, "manual.example")
	require.NoError(t, err)
	assert.True(t, entry.IsPermanent())

	addrs, err := c.LookupHost(ctx, "manual.example")
	require.NoError(t, err)
	assert.Equal(t, "198.51.100.1", addrs[0].String())

	require.NoError(t, c.Delete(ctx, "manual.example"))
	_, err = c.Get(ctx, "manual.example")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestLookupIPAndWarm(t *testing.T) {
	r := newFakeResolver()
	r.set("a.example", "192.0.2.1")
	r.set("b.example", "192.0.2.2")
	c, err := New(Config{TTL: time.Minute, Resolver: r})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	require.NoError(t, c.Warm(ctx, "a.example", "b.example"))
	assert.Equal(t, int64(2), r.lookups.Load())

	ips, err := c.LookupIP(ctx, "a.example")
	require.NoError(t, err)
	require.Len(t, ips, 1)
	assert.Equal(t, "192.0.2.1", ips[0].String())
	assert.Equal(t, int64(2), r.lookups.Load())
}

func TestPurgeAndClose(t *testing.T) {
	r := newFakeResolver()
	r.set("x.example", "192.0.2.1")
	c, err := New(Config{TTL: time.Minute, Resolver: r})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = c.LookupHost(ctx, "x.example")
	require.NoError(t, err)
	require.NoError(t, c.Purge(ctx))
	_, err = c.Get(ctx, "x.example")
	assert.ErrorIs(t, err, ErrNotFound)

	c.Close()
	_, err = c.LookupHost(ctx, "x.example")
	assert.ErrorIs(t, err, ErrClosed)
}

func TestNetworkFilterOnLookup(t *testing.T) {
	r := newFakeResolver()
	r.set("dual.example", "192.0.2.1", "2001:db8::1")
	c, err := New(Config{TTL: time.Minute, Resolver: r, Network: "ip4"})
	require.NoError(t, err)
	defer c.Close()

	addrs, err := c.LookupHost(context.Background(), "dual.example")
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.True(t, addrs[0].Is4())
}

func TestTTLZeroDoesNotAutoCache(t *testing.T) {
	r := newFakeResolver()
	r.set("nocache.example", "192.0.2.1")
	c, err := New(Config{TTL: 0, Resolver: r})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	_, err = c.LookupHost(ctx, "nocache.example")
	require.NoError(t, err)
	_, err = c.LookupHost(ctx, "nocache.example")
	require.NoError(t, err)
	assert.Equal(t, int64(2), r.lookups.Load())
}

func TestOnEventLookupAndHit(t *testing.T) {
	r := newFakeResolver()
	r.set("ev.example", "192.0.2.1")
	var hits atomic.Int64
	var lookups atomic.Int64
	c, err := New(Config{
		TTL:      time.Minute,
		Resolver: r,
		OnEvent: func(ev Event) {
			switch ev.Kind {
			case EventCacheHit:
				hits.Add(1)
			case EventLookup:
				lookups.Add(1)
			}
		},
	})
	require.NoError(t, err)
	defer c.Close()

	ctx := context.Background()
	_, err = c.LookupHost(ctx, "ev.example")
	require.NoError(t, err)
	_, err = c.LookupHost(ctx, "ev.example")
	require.NoError(t, err)
	assert.Equal(t, int64(1), lookups.Load())
	assert.Equal(t, int64(1), hits.Load())
}
