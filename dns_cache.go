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
var defaultTimeout = 10 * time.Second

const (
	PolicyFirst = iota
	PolicyRandom
)

const (
	NetworkIP = iota
	NetworkIPV4
	NetworkIPV6
)

var ErrNotFound = errors.New("Not Found")

type Storage interface {
	Set(host string, ipCache IPCache, ttl time.Duration) error
	Delete(host string) error
	Get(host string) (IPCache, error)
}

type mapStorage struct {
	m sync.Map
}

func (s *mapStorage) Set(host string, ipCache IPCache, _ time.Duration) error {
	s.m.Store(host, &ipCache)
	return nil
}

func (s *mapStorage) Delete(host string) error {
	s.m.Delete(host)
	return nil
}

func (s *mapStorage) Get(host string) (IPCache, error) {
	v, _ := s.m.Load(host)
	if v == nil {
		return IPCache{}, ErrNotFound
	}
	c, ok := v.(*IPCache)
	if !ok {
		return IPCache{}, ErrNotFound
	}
	return *c, nil
}

type (
	// OnStats on stats function
	OnStats func(host string, d time.Duration, ipAddrs []string)
	// DNSCache dns cache
	DNSCache struct {
		Storage  Storage
		TTL      time.Duration
		Stale    time.Duration
		OnStats  OnStats
		Dialer   *net.Dialer
		Resolver *net.Resolver
		Policy   int
		Network  int
	}
	// IPCache ip cache
	IPCache struct {
		IPAddrs   []net.IP
		CreatedAt time.Time
	}
	DNSCacheOption func(*DNSCache)
)

// New create a dns cache instance
func New(ttl time.Duration, opts ...DNSCacheOption) *DNSCache {
	dc := &DNSCache{
		TTL: ttl,
	}
	for _, opt := range opts {
		opt(dc)
	}
	if dc.Storage == nil {
		StorageOption(&mapStorage{})(dc)
	}
	return dc
}

// StorageOption sets storage
func StorageOption(storage Storage) DNSCacheOption {
	return func(d *DNSCache) {
		d.Storage = storage
	}
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

// NetworkOption set network option
func NetworkOption(network int) DNSCacheOption {
	return func(d *DNSCache) {
		d.Network = network
	}
}

// OnStatsOption sets on stats function
func OnStatsOption(onStats OnStats) DNSCacheOption {
	return func(d *DNSCache) {
		d.OnStats = onStats
	}
}

func filterIPByLen(ipAddrs []net.IP, length int) []net.IP {
	if length == 0 {
		return ipAddrs
	}
	ipList := make([]net.IP, 0, len(ipAddrs))
	for _, item := range ipAddrs {
		if len(item) == length {
			ipList = append(ipList, item)
			continue
		}
		// 尝试转换为ipv4
		if length != net.IPv4len {
			continue
		}
		if ip := item.To4(); ip != nil {
			ipList = append(ipList, ip)
			continue
		}
	}
	return ipList
}

func (dc *DNSCache) getIP(ctx context.Context, network, host string) (string, error) {
	// ipv6的host地址会添加[]
	if host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	// 如果已经是ip，直接不解析域名
	if len(net.ParseIP(host)) != 0 {
		return host, nil
	}
	ipAddrs, err := dc.LookupWithCache(ctx, host)
	if err != nil {
		return "", err
	}
	ipLength := 0

	switch network {
	case "tcp4":
		fallthrough
	case "udp4":
		ipLength = net.IPv4len
	case "tcp6":
		fallthrough
	case "udp6":
		ipLength = net.IPv6len
	}
	if ipLength != 0 {
		ipAddrs = filterIPByLen(ipAddrs, ipLength)
	}

	if len(ipAddrs) == 0 {
		return "", ErrNotFound
	}
	index := 0
	if dc.Policy == PolicyRandom {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		index = r.Int() % len(ipAddrs)
	}
	ip := ipAddrs[index]
	return ip.String(), nil
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
		ip, err := dc.getIP(ctx, network, host)
		if err != nil {
			return nil, err
		}
		// IPV6
		if strings.Contains(ip, ":") {
			ip = "[" + ip + "]"
		}
		addr = ip + addr[sepIndex:]
		// TODO 后续确认是否实现dialParallel(ipv4 ipv6一起dial)
		return dialer.DialContext(ctx, network, addr)
	}
}

func (dc *DNSCache) lookup(ctx context.Context, host string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resolver := dc.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	network := "ip"
	switch dc.Network {
	case NetworkIPV4:
		network = "ip4"
	case NetworkIPV6:
		network = "ip6"
	}
	result, err := resolver.LookupIP(ctx, network, host)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return result, nil
}

// Lookup lookup
func (dc *DNSCache) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	start := time.Now()
	ipList, err := dc.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	ipAddrs := make([]string, len(ipList))
	for index, item := range ipList {
		ipAddrs[index] = item.String()
	}
	// 成功则回调
	if dc.OnStats != nil {
		d := time.Since(start)
		dc.OnStats(host, d, ipAddrs)
	}
	return ipList, nil
}

func (dc *DNSCache) lookupAndUpdate(ctx context.Context, host string) ([]net.IP, error) {
	ipAddrs, err := dc.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	err = dc.Set(host, IPCache{
		IPAddrs:   ipAddrs,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, err
	}
	return ipAddrs, nil
}

// LookupWithCache lookup with cache
func (dc *DNSCache) LookupWithCache(ctx context.Context, host string) ([]net.IP, error) {
	ipCache, _ := dc.Get(host)
	if len(ipCache.IPAddrs) != 0 {
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
func (dc *DNSCache) Set(host string, ipCache IPCache) error {
	return dc.Storage.Set(host, ipCache, dc.TTL)
}

// Removes cache of host
func (dc *DNSCache) Delete(host string) error {
	return dc.Storage.Delete(host)
}

// Gets ip cache of host
func (dc *DNSCache) Get(host string) (IPCache, error) {
	return dc.Storage.Get(host)
}
