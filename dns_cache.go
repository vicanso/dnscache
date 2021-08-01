package dnscache

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	defaultDialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

var ErrNotFound = errors.New("Not Found")

type (
	// OnStats on stats function
	OnStats func(host string, d time.Duration, ipAddr net.IPAddr)
	// DNSCache dns cache
	DNSCache struct {
		Caches  *sync.Map
		TTL     time.Duration
		OnStats OnStats
		Dialer  *net.Dialer
	}
	// IPCache ip cache
	IPCache struct {
		IPAddr    net.IPAddr
		CreatedAt time.Time
	}
)

// New create a dns cache instance
func New(ttl time.Duration) *DNSCache {
	return &DNSCache{
		TTL:    ttl,
		Caches: &sync.Map{},
	}
}

// GetDialContext get dial context function with cache
func (dc *DNSCache) GetDialContext() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := defaultDialer
		if dc.Dialer != nil {
			dialer = dc.Dialer
		}
		sepIndex := strings.LastIndex(addr, ":")
		host := addr[:sepIndex]
		ipAddr, err := dc.LookupWithCache(host)
		if err != nil {
			return nil, err
		}
		addr = ipAddr.String() + addr[sepIndex:]
		return dialer.DialContext(ctx, network, addr)
	}
}

// Lookup lookup
func (dc *DNSCache) Lookup(host string) (net.IPAddr, error) {
	start := time.Now()
	ipAddr, err := net.ResolveIPAddr("", host)
	if err != nil {
		return net.IPAddr{}, err
	}
	if ipAddr == nil {
		return net.IPAddr{}, ErrNotFound
	}
	// 成功则回调
	if dc.OnStats != nil {
		d := time.Since(start)
		dc.OnStats(host, d, *ipAddr)
	}
	return *ipAddr, nil
}

// LookupWithCache lookup with cache
func (dc *DNSCache) LookupWithCache(host string) (net.IPAddr, error) {
	ipCache, _ := dc.get(host)
	if ipCache != nil {
		ipAddr := ipCache.IPAddr
		createdAt := ipCache.CreatedAt
		// 如果创建时间小于0，表示永久有效
		// 如果在有效期内，直接返回
		if createdAt.IsZero() || createdAt.Add(dc.TTL).After(time.Now()) {
			return ipAddr, nil
		}
	}
	ipAddr, err := dc.Lookup(host)
	if err != nil {
		return net.IPAddr{}, err
	}
	dc.Set(host, IPCache{
		IPAddr:    ipAddr,
		CreatedAt: time.Now(),
	})
	return ipAddr, nil
}

// Sets ip cache for the host
func (dc *DNSCache) Set(host string, ipCache IPCache) {
	dc.Caches.Store(host, &ipCache)
}

// Removes cache of host
func (dc *DNSCache) Remove(host string) {
	dc.Caches.Delete(host)
}

func (dc *DNSCache) get(host string) (*IPCache, bool) {
	v, _ := dc.Caches.Load(host)
	if v == nil {
		return nil, false
	}
	c, ok := v.(*IPCache)
	return c, ok
}

// Gets ip cache of host
func (dc *DNSCache) Get(host string) (IPCache, bool) {
	c, ok := dc.get(host)
	if !ok {
		return IPCache{}, false
	}
	return *c, true
}
