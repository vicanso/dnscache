package dnscache

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startLocalListener(t *testing.T) (addr string, closeFn func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestDialContextSuccess(t *testing.T) {
	addr, closeFn := startLocalListener(t)
	defer closeFn()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	r := newFakeResolver()
	r.set("dial.example", host)
	c, err := New(Config{
		TTL:      time.Minute,
		Resolver: r,
		Policy:   PolicyFirst,
		Dialer:   &net.Dialer{Timeout: time.Second},
	})
	require.NoError(t, err)
	defer c.Close()

	conn, err := c.DialContext(context.Background(), "tcp", net.JoinHostPort("dial.example", port))
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()
}

func TestDialContextTriesNextAddress(t *testing.T) {
	goodAddr, closeFn := startLocalListener(t)
	defer closeFn()
	goodHost, goodPort, err := net.SplitHostPort(goodAddr)
	require.NoError(t, err)

	var attempts atomic.Int64
	c, err := New(Config{
		TTL: time.Minute,
		Resolver: &funcResolver{fn: func(ctx context.Context, network, host string) ([]net.IP, error) {
			// First candidate is documentation-range (fails); second is local listener.
			return []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP(goodHost)}, nil
		}},
		Policy:      PolicyFirst,
		DialTimeout: 100 * time.Millisecond,
		Dialer:      &net.Dialer{},
		OnEvent: func(ev Event) {
			if ev.Kind == EventDial {
				attempts.Add(1)
			}
		},
	})
	require.NoError(t, err)
	defer c.Close()

	conn, err := c.DialContext(context.Background(), "tcp", net.JoinHostPort("multi.example", goodPort))
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()
	assert.GreaterOrEqual(t, attempts.Load(), int64(2))
}

type funcResolver struct {
	fn func(ctx context.Context, network, host string) ([]net.IP, error)
}

func (f *funcResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	return f.fn(ctx, network, host)
}

func TestDialContextNoAddresses(t *testing.T) {
	r := newFakeResolver()
	r.set("v6only.example", "2001:db8::1")
	c, err := New(Config{
		TTL:      time.Minute,
		Resolver: r,
		Policy:   PolicyFirst,
	})
	require.NoError(t, err)
	defer c.Close()

	_, err = c.DialContext(context.Background(), "tcp4", "v6only.example:80")
	assert.ErrorIs(t, err, ErrNoAddresses)
}

func TestDialContextFallbackToSystem(t *testing.T) {
	addr, closeFn := startLocalListener(t)
	defer closeFn()

	r := newFakeResolver()
	r.setErr(errors.New("resolver down"))
	c, err := New(Config{
		TTL:              time.Minute,
		Resolver:         r,
		FallbackToSystem: true,
		Dialer:           &net.Dialer{Timeout: time.Second},
	})
	require.NoError(t, err)
	defer c.Close()

	// Dial original address directly via system path on cache failure.
	conn, err := c.DialContext(context.Background(), "tcp", addr)
	require.NoError(t, err)
	_ = conn.Close()
}

func TestDialContextInvalidAddress(t *testing.T) {
	c, err := New(Config{TTL: time.Minute, Resolver: newFakeResolver()})
	require.NoError(t, err)
	defer c.Close()
	_, err = c.DialContext(context.Background(), "tcp", "not-a-valid-address")
	require.Error(t, err)
}

func TestDialContextRespectsCancel(t *testing.T) {
	r := newFakeResolver()
	r.set("slow.example", "192.0.2.1")
	c, err := New(Config{
		TTL:         time.Minute,
		Resolver:    r,
		Policy:      PolicyFirst,
		DialTimeout: 2 * time.Second,
		Dialer:      &net.Dialer{},
	})
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.DialContext(ctx, "tcp", "slow.example:80")
	require.Error(t, err)
}

func TestDialContextJoinsErrors(t *testing.T) {
	r := newFakeResolver()
	r.set("dead.example", "192.0.2.1", "192.0.2.2")
	c, err := New(Config{
		TTL:         time.Minute,
		Resolver:    r,
		Policy:      PolicyFirst,
		DialTimeout: 50 * time.Millisecond,
		Dialer:      &net.Dialer{},
	})
	require.NoError(t, err)
	defer c.Close()

	_, err = c.DialContext(context.Background(), "tcp", "dead.example:1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "192.0.2.1")
	assert.Contains(t, err.Error(), "192.0.2.2")
}
