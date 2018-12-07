# DNS Cache

[![Build Status](https://travis-ci.org/vicanso/dns-cache.svg?branch=master)](https://travis-ci.org/vicanso/dns-cache)

DNS Cache for http client

## API

### New

create a dns cache instance

- `ttl` cache's ttl seconds

```go
ds := dnscache.New(60)
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