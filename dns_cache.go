package dnscache

import (
	"context"
	"errors"
	"math/rand"
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

const (
	PolicyFirst = iota
	PolicyRandom
)

var ErrNotFound = errors.New("Not Found")

type (
	// OnStats on stats function
	OnStats func(host string, d time.Duration, ipAddrs []string)
	// DNSCache dns cache
	DNSCache struct {
		Caches   *sync.Map
		TTL      time.Duration
		OnStats  OnStats
		Dialer   *net.Dialer
		Resolver *net.Resolver
		Policy   int
	}
	// IPCache ip cache
	IPCache struct {
		IPAddrs   []string
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
		ipAddrs, err := dc.LookupWithCache(host)
		if err != nil {
			return nil, err
		}
		if len(ipAddrs) == 0 {
			return nil, ErrNotFound
		}
		index := 0
		if dc.Policy == PolicyRandom {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			index = r.Int() % len(ipAddrs)
		}
		// 选择第一个解析IP，后续再看是否增加更多的处理
		addr = ipAddrs[index] + addr[sepIndex:]
		return dialer.DialContext(ctx, network, addr)
	}
}

// Lookup lookup
func (dc *DNSCache) Lookup(host string) ([]string, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resolver := dc.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	result, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	ipAddrs := make([]string, len(result))
	for index, item := range result {
		ipAddrs[index] = item.String()
	}
	// 成功则回调
	if dc.OnStats != nil {
		d := time.Since(start)
		dc.OnStats(host, d, ipAddrs)
	}
	return ipAddrs, nil
}

// LookupWithCache lookup with cache
func (dc *DNSCache) LookupWithCache(host string) ([]string, error) {
	ipCache, _ := dc.get(host)
	if ipCache != nil {
		ipAddrs := ipCache.IPAddrs
		createdAt := ipCache.CreatedAt
		// 如果创建时间小于0，表示永久有效
		// 如果在有效期内，直接返回
		if createdAt.IsZero() || createdAt.Add(dc.TTL).After(time.Now()) {
			return ipAddrs, nil
		}
	}
	ipAddrs, err := dc.Lookup(host)
	if err != nil {
		return nil, err
	}
	dc.Set(host, IPCache{
		IPAddrs:   ipAddrs,
		CreatedAt: time.Now(),
	})
	return ipAddrs, nil
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
