package dnscache

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

type (
	// DNSCache dns cache
	DNSCache struct {
		Caches *sync.Map
		TTL    int64
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
		sepIndex := strings.LastIndex(addr, ":")
		host := addr[:sepIndex]
		ipAddr, err := ds.LookupWithCache(host)
		if err != nil {
			return nil, err
		}
		addr = ipAddr.String() + addr[sepIndex:]
		dialer := &net.Dialer{}
		return dialer.DialContext(ctx, network, addr)
	}
}

// Lookup lookup
func (ds *DNSCache) Lookup(host string) (*net.IPAddr, error) {
	return net.ResolveIPAddr("", host)
}

// LookupWithCache lookup with cache
func (ds *DNSCache) LookupWithCache(host string) (ipAddr *net.IPAddr, err error) {
	v, ok := ds.Caches.Load(host)
	if ok && v != nil {
		ipCache := v.(*IPCache)
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
	ds.Caches.Store(host, &IPCache{
		IPAddr:    ipAddr,
		CreatedAt: time.Now().Unix(),
	})
	return
}

// Add add ip cache
func (ds *DNSCache) Add(host string, ipCache *IPCache) {
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
