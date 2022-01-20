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
var defaultTimeout = 3 * time.Second

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
		Stale    time.Duration
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
	DNSCacheOption func(*DNSCache)
)

// New create a dns cache instance
func New(ttl time.Duration, opts ...DNSCacheOption) *DNSCache {
	ds := &DNSCache{
		TTL:    ttl,
		Caches: &sync.Map{},
	}
	for _, opt := range opts {
		opt(ds)
	}
	return ds
}

// PolicyOption sets policy
func PolicyOption(policy int) DNSCacheOption {
	return func(d *DNSCache) {
		d.Policy = policy
	}
}

// StaleOption sets stale
func StaleOption(stale time.Duration) DNSCacheOption {
	return func(d *DNSCache) {
		d.Stale = stale
	}
}

// DialerOption sets dialer
func DialerOption(dialer *net.Dialer) DNSCacheOption {
	return func(d *DNSCache) {
		d.Dialer = dialer
	}
}

// ResolverOption sets resolver
func ResolverOption(resolver *net.Resolver) DNSCacheOption {
	return func(d *DNSCache) {
		d.Resolver = resolver
	}
}

// OnStatsOption sets on stats function
func OnStatsOption(onStats OnStats) DNSCacheOption {
	return func(d *DNSCache) {
		d.OnStats = onStats
	}
}

func isIP(host string) bool {
	if len(host) < 2 {
		return false
	}
	if host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return len(net.ParseIP(host)) != 0
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
		// 如果已经是ip，直接不解析域名
		if isIP(host) {
			return dialer.DialContext(ctx, network, addr)
		}
		ipAddrs, err := dc.LookupWithCache(ctx, host)
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
		ip := ipAddrs[index]
		// IPV6
		if strings.Contains(ip, ":") {
			ip = "[" + ip + "]"
		}
		// 选择第一个解析IP，后续再看是否增加更多的处理
		addr = ip + addr[sepIndex:]
		return dialer.DialContext(ctx, network, addr)
	}
}

// Lookup lookup
func (dc *DNSCache) Lookup(ctx context.Context, host string) ([]string, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
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

func (dc *DNSCache) lookupAndUpdate(ctx context.Context, host string) ([]string, error) {
	ipAddrs, err := dc.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	dc.Set(host, IPCache{
		IPAddrs:   ipAddrs,
		CreatedAt: time.Now(),
	})
	return ipAddrs, nil
}

// LookupWithCache lookup with cache
func (dc *DNSCache) LookupWithCache(ctx context.Context, host string) ([]string, error) {
	ipCache, _ := dc.get(host)
	if ipCache != nil {
		ipAddrs := ipCache.IPAddrs
		createdAt := ipCache.CreatedAt
		now := time.Now()
		// 如果创建时间为0，表示永久有效
		// 如果在有效期内，直接返回
		if createdAt.IsZero() || createdAt.Add(dc.TTL).After(now) {
			return ipAddrs, nil
		}
		// 如果加上stale时长还未过期，则可以直接返回并更新dns解析
		if dc.Stale != 0 && createdAt.Add(dc.TTL).Add(dc.Stale).After(now) {
			// dns本身的更新是singleflight
			// 因此此处暂不控制并发
			go func() {
				_, _ = dc.lookupAndUpdate(context.Background(), host)
			}()
			return ipAddrs, nil
		}
	}
	return dc.lookupAndUpdate(ctx, host)
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
