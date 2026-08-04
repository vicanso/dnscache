package dnscache

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// fakeResolver is a test double for IPResolver.
type fakeResolver struct {
	mu      sync.Mutex
	hosts   map[string][]net.IP
	err     error
	delay   time.Duration
	lookups atomic.Int64
}

func newFakeResolver() *fakeResolver {
	return &fakeResolver{
		hosts: make(map[string][]net.IP),
	}
}

func (f *fakeResolver) set(host string, ips ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]net.IP, 0, len(ips))
	for _, s := range ips {
		out = append(out, net.ParseIP(s))
	}
	f.hosts[normalizeHost(host)] = out
}

func (f *fakeResolver) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	f.lookups.Add(1)
	f.mu.Lock()
	delay := f.delay
	err := f.err
	ips := f.hosts[normalizeHost(host)]
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("no such host")
	}

	// optional family filter like the real resolver
	if network == "ip4" || network == "ip6" {
		filtered := make([]net.IP, 0, len(ips))
		for _, ip := range ips {
			if network == "ip4" && ip.To4() != nil {
				filtered = append(filtered, ip.To4())
			}
			if network == "ip6" && ip.To4() == nil {
				filtered = append(filtered, ip)
			}
		}
		ips = filtered
	}
	if len(ips) == 0 {
		return nil, errors.New("no such host")
	}
	out := make([]net.IP, len(ips))
	copy(out, ips)
	return out, nil
}

