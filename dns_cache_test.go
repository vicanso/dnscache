package dnscache

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDNSCache(t *testing.T) {
	assert := assert.New(t)

	dialer := &net.Dialer{}
	resolver := &net.Resolver{}
	onStats := func(host string, d time.Duration, ipAddrs []string) {
		fmt.Println(host)
	}
	dc := New(
		time.Minute,
		PolicyOption(PolicyRandom),
		StaleOption(time.Second),
		DialerOption(dialer),
		ResolverOption(resolver),
		OnStatsOption(onStats),
	)
	assert.Equal(time.Minute, dc.TTL)
	assert.Equal(PolicyRandom, dc.Policy)
	assert.Equal(time.Second, dc.Stale)
	assert.Equal(dialer, dc.Dialer)
	assert.Equal(resolver, dc.Resolver)
}

func TestLookup(t *testing.T) {
	assert := assert.New(t)
	dc := New(0)
	dc.Policy = PolicyRandom
	ipAddr, err := dc.Lookup(context.Background(), "www.bing.com")
	assert.Nil(err)
	assert.NotEmpty(ipAddr)
}

func TestLookupWithCache(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.bing.com"
	ipAddr, err := dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)
	assert.NotEmpty(ipAddr)

	_, err = dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)

	dc = New(time.Second)
	count := int32(0)

	dc.OnStats = func(host string, d time.Duration, ipAddrs []string) {
		atomic.AddInt32(&count, 1)
	}
	dc.Stale = 3 * time.Second
	host = "www.bing.com"
	ipAddr, err = dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)
	assert.NotEmpty(ipAddr)
	time.Sleep(2 * time.Second)
	ipAddr1, err := dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)
	assert.Equal(ipAddr, ipAddr1)
	assert.Equal(int32(1), atomic.LoadInt32(&count))
	// go routine更新后，count变为2
	time.Sleep(2 * time.Second)
	assert.Equal(int32(2), atomic.LoadInt32(&count))
}

func TestOnStats(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.baidu.com"
	done := false
	dc.OnStats = func(h string, d time.Duration, _ []string) {
		assert.NotEmpty(d.Nanoseconds())
		assert.Equal(host, h)
		done = true
	}
	_, err := dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)
	assert.True(done)
}

func TestGetDialContext(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	dc.Dialer = &net.Dialer{}
	dc.Policy = PolicyRandom
	http.DefaultClient.Transport = &http.Transport{
		DialContext: dc.GetDialContext(),
	}
	resp, err := http.Get("https://www.baidu.com/")
	assert.Nil(err)
	assert.Equal(200, resp.StatusCode)
}

func TestSetCache(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.baidu.com"
	dc.Set(host, IPCache{
		CreatedAt: time.Now(),
		IPAddrs: []string{
			"1.1.1.1",
		},
	})
	ipAddrs, err := dc.LookupWithCache(context.Background(), host)
	assert.Nil(err)
	assert.Equal([]string{"1.1.1.1"}, ipAddrs)
	_, ok := dc.Get(host)
	assert.True(ok)
	dc.Remove(host)
	_, ok = dc.Get(host)
	assert.False(ok)
}

func BenchmarkLookupWithCache(b *testing.B) {
	dc := New(time.Minute)
	_, _ = dc.LookupWithCache(context.Background(), "www.baidu.com")
	for i := 0; i < b.N; i++ {
		_, _ = dc.LookupWithCache(context.Background(), "www.baidu.com")
	}
}

func BenchmarkDial(b *testing.B) {
	dc := New(time.Minute)
	fn := dc.GetDialContext()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, _ = fn(ctx, "tcp", "www.baidu.com:443")
	}
}
