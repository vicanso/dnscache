# DNS Cache

[![Build Status](https://github.com/vicanso/dnscache/workflows/Test/badge.svg)](https://github.com/vicanso/dnscache/actions)

DNS Cache for http client

## API

### New

create a dns cache instance

- `ttl` cache's ttl seconds

```go
onStats := func(h string, d time.Duration, _ []string) {
  fmt.Println(d)
}
dc := dnscache.New(
  time.Minute,
  dnscache.PolicyOption(dnscache.PolicyRandom),
  dnscache.StaleOption(time.Minute),
  dnscache.OnStatsOption(onStats),
)
```

### Lookup

lookup ip address for host

- `host` host name

```go
dc := dnscache.New(time.Minute)
ipAddrs, err := dc.Lookup(context.Background(), "www.baidu.com")
```

### LookupWithCache

lookup ip address for host, it will use the cache first.

- `host` host name

```go
dc := dnscache.New(time.Minute)
ipAddrs, err := dc.LookupWithCache(context.Background(), "www.baidu.com")
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
  IPAddrs: []string{
    "1.1.1.1",
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
  IPAddrs: []string{
    "1.1.1.1",
  },
})
cache, ok := dc.Get(host)
```