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
	ipAddr, err := dc.Lookup("www.baidu.com")
	assert.Nil(err)
	assert.NotEmpty(ipAddr.String())
}

func TestLookupWithCache(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.baidu.com"
	ipAddr, err := dc.LookupWithCache(host)
	assert.Nil(err)
	assert.NotEmpty(ipAddr.String())

	_, err = dc.LookupWithCache(host)
	assert.Nil(err)
}

func TestOnStats(t *testing.T) {
	assert := assert.New(t)
	dc := New(time.Minute)
	host := "www.baidu.com"
	done := false
	dc.OnStats = func(h string, d time.Duration, _ *net.IPAddr) {
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
	dc.Set(host, &IPCache{
		CreatedAt: time.Now(),
		IPAddr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})
	ipAddr, err := dc.LookupWithCache(host)
	assert.Nil(err)
	assert.Equal("1.1.1.1", ipAddr.String())
	assert.NotNil(dc.Get(host))
	dc.Remove(host)
	assert.Nil(dc.Get(host))
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
