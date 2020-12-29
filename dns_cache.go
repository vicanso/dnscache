package dnscache

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	defaultDialer = &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
)

type (
	// OnStats on stats function
	OnStats func(host string, d time.Duration, ipAddr *net.IPAddr)
	// DNSCache dns cache
	DNSCache struct {
		Caches  *sync.Map
		TTL     time.Duration
		OnStats OnStats
		Dialer  *net.Dialer
	}
	// IPCache ip cache
	IPCache struct {
		IPAddr    *net.IPAddr
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
func (dc *DNSCache) Lookup(host string) (ipAddr *net.IPAddr, err error) {
	start := time.Now()
	ipAddr, err = net.ResolveIPAddr("", host)
	d := time.Since(start)
	if dc.OnStats != nil {
		dc.OnStats(host, d, ipAddr)
	}
	return
}

// LookupWithCache lookup with cache
func (dc *DNSCache) LookupWithCache(host string) (ipAddr *net.IPAddr, err error) {
	ipCache := dc.Get(host)
	if ipCache != nil {
		ipAddr = ipCache.IPAddr
		createdAt := ipCache.CreatedAt
		// 如果创建时间小于0，表示永久有效
		if createdAt.IsZero() {
			return
		}
		// 如果在有效期内，直接返回
		if createdAt.Add(dc.TTL).After(time.Now()) {
			return
		}
	}
	ipAddr, err = dc.Lookup(host)
	if err != nil {
		return
	}
	dc.Set(host, &IPCache{
		IPAddr:    ipAddr,
		CreatedAt: time.Now(),
	})
	return
}

// Set set ip cache
func (dc *DNSCache) Set(host string, ipCache *IPCache) {
	dc.Caches.Store(host, ipCache)
}

// Remove remove cache
func (dc *DNSCache) Remove(host string) {
	dc.Caches.Delete(host)
}

// Get get ip cache
func (dc *DNSCache) Get(host string) *IPCache {
	v, _ := dc.Caches.Load(host)
	if v == nil {
		return nil
	}
	return v.(*IPCache)
}
