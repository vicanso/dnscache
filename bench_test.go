package dnscache

import (
	"context"
	"testing"
	"time"
)

func BenchmarkLookupHost(b *testing.B) {
	r := newFakeResolver()
	r.set("bench.example", "192.0.2.1", "2001:db8::1")
	c, err := New(Config{TTL: time.Minute, Resolver: r})
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	if _, err := c.LookupHost(ctx, "bench.example"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.LookupHost(ctx, "bench.example"); err != nil {
			b.Fatal(err)
		}
	}
}
