package dnscache

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntryFreshStaleExpired(t *testing.T) {
	now := time.Now()
	e := Entry{
		Addrs:      []netip.Addr{mustAddr("192.0.2.1")},
		FreshUntil: now.Add(time.Second),
		StaleUntil: now.Add(3 * time.Second),
	}
	assert.True(t, e.IsFresh(now))
	assert.False(t, e.IsStale(now))
	assert.False(t, e.IsExpired(now))

	assert.False(t, e.IsFresh(now.Add(2*time.Second)))
	assert.True(t, e.IsStale(now.Add(2*time.Second)))
	assert.False(t, e.IsExpired(now.Add(2*time.Second)))

	assert.True(t, e.IsExpired(now.Add(4*time.Second)))
}

func TestEntryPermanentAndNegative(t *testing.T) {
	p := Permanent(mustAddr("192.0.2.1"))
	assert.True(t, p.IsPermanent())
	assert.True(t, p.IsFresh(time.Now()))
	assert.False(t, p.IsStale(time.Now()))

	n := Entry{Negative: true, FreshUntil: time.Now().Add(time.Minute)}
	assert.True(t, n.IsFresh(time.Now()))
	assert.False(t, n.IsStale(time.Now()))
	assert.False(t, n.IsFresh(time.Now().Add(2*time.Minute)))
}
