# DNS Cache

[![Build Status](https://travis-ci.org/vicanso/dnscache.svg?branch=master)](https://travis-ci.org/vicanso/dnscache)

DNS Cache for http client

## API

### New

create a dns cache instance

- `ttl` cache's ttl seconds

```go
ds := dnscache.New(60)
```

### OnStats

```go
ds := dnscache.New(60)
ds.OnStats = func(h string, d time.Duration, _ *net.IPAddr) {
  fmt.Println(d)
}
ds.LookupWithCache("www.baidu.com")
```

### Lookup

lookup ip address for host

- `host` host name

```go
ds := dnscache.New(60)
ipAddr, err := ds.Lookup("www.baidu.com")
```

### LookupWithCache

lookup ip address for host, it will use the cache first.

- `host` host name

```go
ds := dnscache.New(60)
ipAddr, err := ds.LookupWithCache("www.baidu.com")
```

### GetDialContext

get dial context function for http client

```go
ds := New(60)
http.DefaultClient.Transport = &http.Transport{
  DialContext: ds.GetDialContext(),
}
resp, err := http.Get("https://www.baidu.com/")
```

### Set

set the dns cache, if the `CreatedAt` is less than 0, it will never be expired.

```go
ds := New(60)
ds.Set("www.baidu.com", &IPCache{
  CreatedAt: time.Now().Unix(),
  IPAddr: &net.IPAddr{
    IP: net.IPv4(1, 1, 1, 1),
  },
})
```

### Get

get the dns cache

```go
ds := New(60)
host := "www.baidu.com"
ds.Set(host, &IPCache{
  CreatedAt: time.Now().Unix(),
  IPAddr: &net.IPAddr{
    IP: net.IPv4(1, 1, 1, 1),
  },
})
cache := ds.Get(host)
```