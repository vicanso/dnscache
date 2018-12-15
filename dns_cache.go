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
	// OnStats onstats function
	OnStats func(host string, d time.Duration, ipAddr *net.IPAddr)
	// DNSCache dns cache
	DNSCache struct {
		Caches  *sync.Map
		TTL     int64
		OnStats OnStats
		Dialer  *net.Dialer
	}
	// IPCache ip cache
	IPCache struct {
		IPAddr    *net.IPAddr
		CreatedAt int64
	}
)

// New create a dns cache instance
func New(ttl int64) *DNSCache {
	return &DNSCache{
		TTL:    ttl,
		Caches: new(sync.Map),
	}
}

// GetDialContext get dial context function
func (ds *DNSCache) GetDialContext() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := defaultDialer
		if ds.Dialer != nil {
			dialer = ds.Dialer
		}
		sepIndex := strings.LastIndex(addr, ":")
		host := addr[:sepIndex]
		ipAddr, err := ds.LookupWithCache(host)
		if err != nil {
			return nil, err
		}
		addr = ipAddr.String() + addr[sepIndex:]
		return dialer.DialContext(ctx, network, addr)
	}
}

// Lookup lookup
func (ds *DNSCache) Lookup(host string) (ipAddr *net.IPAddr, err error) {
	start := time.Now()
	ipAddr, err = net.ResolveIPAddr("", host)
	d := time.Since(start)
	if ds.OnStats != nil {
		ds.OnStats(host, d, ipAddr)
	}
	return
}

// LookupWithCache lookup with cache
func (ds *DNSCache) LookupWithCache(host string) (ipAddr *net.IPAddr, err error) {
	ipCache := ds.Get(host)
	if ipCache != nil {
		ipAddr = ipCache.IPAddr
		createdAt := ipCache.CreatedAt
		// 如果创建时间小于0，表示永久有效
		if createdAt < 0 {
			return
		}
		// 如果在有效期内，直接返回
		s := time.Now().Unix() - createdAt
		if s < ds.TTL {
			return
		}
	}
	ipAddr, err = ds.Lookup(host)
	if err != nil {
		return
	}
	ds.Set(host, &IPCache{
		IPAddr:    ipAddr,
		CreatedAt: time.Now().Unix(),
	})
	return
}

// Set set ip cache
func (ds *DNSCache) Set(host string, ipCache *IPCache) {
	ds.Caches.Store(host, ipCache)
}

// Remove remove cache
func (ds *DNSCache) Remove(host string) {
	ds.Caches.Delete(host)
}

// Get get ip cache
func (ds *DNSCache) Get(host string) *IPCache {
	v, _ := ds.Caches.Load(host)
	if v == nil {
		return nil
	}
	return v.(*IPCache)
}
