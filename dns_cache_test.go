package dnscache

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	assert := assert.New(t)
	dc := New(0)
	dc.Policy = PolicyRandom
	ipAddr, err := dc.Lookup("www.bing.com")
	assert.Nil(err)
	assert.NotEmpty(ipAddr)
}

func TestLookupWithCache(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.bing.com"
	ipAddr, err := dc.LookupWithCache(host)
	assert.Nil(err)
	assert.NotEmpty(ipAddr)

	_, err = dc.LookupWithCache(host)
	assert.Nil(err)
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
	_, err := dc.LookupWithCache(host)
	assert.Nil(err)
	assert.True(done)
}

func TestGetDialContext(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	dc.Dialer = &net.Dialer{}
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
	ipAddrs, err := dc.LookupWithCache(host)
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
	_, _ = dc.LookupWithCache("www.baidu.com")
	for i := 0; i < b.N; i++ {
		_, _ = dc.LookupWithCache("www.baidu.com")
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
