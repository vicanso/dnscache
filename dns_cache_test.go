package dnscache

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	ds := New(0)
	ipAddr, err := ds.Lookup("www.baidu.com")
	if err != nil {
		t.Fatalf("dns look up fail, %v", err)
	}
	if ipAddr.String() == "" {
		t.Fatalf("dns look up fail")
	}
}

func TestLookupWithCache(t *testing.T) {
	ds := New(60)
	host := "www.baidu.com"
	ipAddr, err := ds.LookupWithCache(host)
	if err != nil {
		t.Fatalf("dns look up with cache fail, %v", err)
	}
	if ipAddr.String() == "" {
		t.Fatalf("dns look up with cache fail")
	}
	_, err = ds.LookupWithCache(host)
	if err != nil {
		t.Fatalf("dns look up from cache fail, %v", err)
	}
}

func TestOnStats(t *testing.T) {
	ds := New(60)
	host := "www.baidu.com"
	done := false
	ds.OnStats = func(h string, d time.Duration, _ *net.IPAddr) {
		if d.Nanoseconds() == 0 || h != host {
			t.Fatalf("get duration on stats fail")
		}
		done = true
	}
	ds.LookupWithCache(host)
	if !done {
		t.Fatalf("get duration on stats fail")
	}

}

func TestGetDialContext(t *testing.T) {
	ds := New(60)
	http.DefaultClient.Transport = &http.Transport{
		DialContext: ds.GetDialContext(),
	}
	resp, err := http.Get("https://www.baidu.com/")
	if err != nil {
		t.Fatalf("http get fail, %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("http get fail, status is %d", resp.StatusCode)
	}
}

func TestAddCache(t *testing.T) {
	ds := New(60)
	host := "www.baidu.com"
	ds.Add(host, &IPCache{
		CreatedAt: time.Now().Unix(),
		IPAddr: &net.IPAddr{
			IP: net.IPv4(1, 1, 1, 1),
		},
	})
	ipAddr, err := ds.LookupWithCache(host)
	if err != nil {
		t.Fatalf("dns lookup fail, %v", err)
	}
	if ipAddr.String() != "1.1.1.1" {
		t.Fatalf("add cache fail")
	}
	if ds.Get(host) == nil {
		t.Fatalf("get cache fail")
	}
	ds.Remove(host)
	if ds.Get(host) != nil {
		t.Fatalf("remove cache fail")
	}
}
