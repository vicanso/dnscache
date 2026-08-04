package dnscache

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// DialContext resolves address via the cache and dials candidates until one succeeds.
// It is safe to use directly as http.Transport.DialContext.
func (c *Cache) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("dnscache: split address: %w", err)
	}

	addrs, err := c.LookupHost(ctx, host)
	if err != nil {
		if c.fallbackToSystem {
			return c.dialer.DialContext(ctx, network, address)
		}
		c.emit(Event{Kind: EventError, Host: host, Network: network, Address: address, Err: err})
		return nil, err
	}

	candidates := filterByNetwork(addrs, network)
	if len(candidates) == 0 {
		err = ErrNoAddresses
		if c.fallbackToSystem {
			return c.dialer.DialContext(ctx, network, address)
		}
		c.emit(Event{Kind: EventError, Host: host, Network: network, Address: address, Err: err})
		return nil, err
	}

	candidates = orderAddrs(candidates, c.policy, c.rotate, normalizeHost(host), &c.hostIndex)

	var errs []error
	for _, addr := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dialAddr := net.JoinHostPort(addr.String(), port)
		start := time.Now()
		conn, dialErr := c.dialOne(ctx, network, dialAddr)
		c.emit(Event{
			Kind:     EventDial,
			Host:     host,
			Network:  network,
			Address:  dialAddr,
			Duration: time.Since(start),
			Addrs:    []netip.Addr{addr},
			Err:      dialErr,
		})
		if dialErr == nil {
			return conn, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", dialAddr, dialErr))
	}
	joined := errors.Join(errs...)
	c.emit(Event{Kind: EventError, Host: host, Network: network, Address: address, Err: joined})
	return nil, joined
}

func (c *Cache) dialOne(ctx context.Context, network, address string) (net.Conn, error) {
	if c.dialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.dialTimeout)
		defer cancel()
	}
	return c.dialer.DialContext(ctx, network, address)
}

// ipToNetIP converts netip.Addr values to net.IP for legacy helpers.
func ipToNetIP(addrs []netip.Addr) []net.IP {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]net.IP, len(addrs))
	for i, a := range addrs {
		out[i] = a.AsSlice()
	}
	return out
}

// netIPToAddr converts net.IP values to netip.Addr, skipping invalids.
func netIPToAddr(ips []net.IP) []netip.Addr {
	if len(ips) == 0 {
		return nil
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		out = append(out, addr.Unmap())
	}
	return dedupeAddrs(out)
}
