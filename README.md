# DNS Cache

[![Build Status](https://github.com/vicanso/dnscache/workflows/Test/badge.svg)](https://github.com/vicanso/dnscache/actions)

DNS Cache for http client

## API

### New

create a dns cache instance

- `ttl` cache's ttl seconds

```go
dc := dnscache.New(time.Minute)
```

### OnStats

```go
dc := dnscache.New(time.Minute)
dc.OnStats = func(h string, d time.Duration, _ net.IPAddr) {
  fmt.Println(d)
}
dc.LookupWithCache("www.baidu.com")
```

### Lookup

lookup ip address for host

- `host` host name

```go
dc := dnscache.New(time.Minute)
ipAddr, err := dc.Lookup("www.baidu.com")
```

### LookupWithCache

lookup ip address for host, it will use the cache first.

- `host` host name

```go
dc := dnscache.New(time.Minute)
ipAddr, err := dc.LookupWithCache("www.baidu.com")
```

### GetDialContext

get dial context function for http client

```go
dc := dnscache.New(time.Minute)
http.DefaultClient.Transport = &http.Transport{
  DialContext: dc.GetDialContext(),
}
resp, err := http.Get("https://www.baidu.com/")
```

### Set

set the dns cache, if the `CreatedAt` is less than 0, it will never be expired.

```go
dc := dnscache.New(time.Minute)
dc.Set("www.baidu.com", IPCache{
  CreatedAt: time.Now().Unix(),
  IPAddr: net.IPAddr{
    IP: net.IPv4(1, 1, 1, 1),
  },
})
```

### Get

get the dns cache

```go
dc := dnscache.New(time.Minute)
host := "www.baidu.com"
dc.Set(host, IPCache{
  CreatedAt: time.Now().Unix(),
  IPAddr: net.IPAddr{
    IP: net.IPv4(1, 1, 1, 1),
  },
})
cache, ok := dc.Get(host)
```