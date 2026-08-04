package dnscache

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

func TestDedupeAddrs(t *testing.T) {
	in := []netip.Addr{
		mustAddr("192.0.2.1"),
		mustAddr("::ffff:192.0.2.1"),
		mustAddr("2001:db8::1"),
		mustAddr("2001:db8::1"),
	}
	out := dedupeAddrs(in)
	require.Len(t, out, 2)
	assert.Equal(t, "192.0.2.1", out[0].String())
	assert.Equal(t, "2001:db8::1", out[1].String())
}

func TestFilterByNetwork(t *testing.T) {
	in := []netip.Addr{mustAddr("192.0.2.1"), mustAddr("2001:db8::1")}
	assert.Equal(t, []netip.Addr{mustAddr("192.0.2.1")}, filterByNetwork(in, "tcp4"))
	assert.Equal(t, []netip.Addr{mustAddr("2001:db8::1")}, filterByNetwork(in, "tcp6"))
	assert.Equal(t, in, filterByNetwork(in, "tcp"))
}

func TestInterleaveFamilies(t *testing.T) {
	in := []netip.Addr{
		mustAddr("2001:db8::1"),
		mustAddr("2001:db8::2"),
		mustAddr("192.0.2.1"),
		mustAddr("192.0.2.2"),
	}
	out := interleaveFamilies(in, false, "h", &hostCounters{})
	assert.Equal(t, []netip.Addr{
		mustAddr("2001:db8::1"),
		mustAddr("192.0.2.1"),
		mustAddr("2001:db8::2"),
		mustAddr("192.0.2.2"),
	}, out)
}

func TestOrderAddrsRoundRobinPerHost(t *testing.T) {
	in := []netip.Addr{mustAddr("192.0.2.1"), mustAddr("192.0.2.2"), mustAddr("192.0.2.3")}
	counters := &hostCounters{}
	a := orderAddrs(in, PolicyRoundRobin, false, "a.example", counters)
	b := orderAddrs(in, PolicyRoundRobin, false, "a.example", counters)
	c := orderAddrs(in, PolicyRoundRobin, false, "b.example", counters)

	assert.Equal(t, mustAddr("192.0.2.1"), a[0])
	assert.Equal(t, mustAddr("192.0.2.2"), b[0])
	// independent host counter starts at 0
	assert.Equal(t, mustAddr("192.0.2.1"), c[0])
}

func TestOrderAddrsFirst(t *testing.T) {
	in := []netip.Addr{mustAddr("192.0.2.1"), mustAddr("192.0.2.2")}
	out := orderAddrs(in, PolicyFirst, false, "h", &hostCounters{})
	assert.Equal(t, in, out)
}

func TestOrderAddrsRandom(t *testing.T) {
	in := []netip.Addr{
		mustAddr("192.0.2.1"),
		mustAddr("192.0.2.2"),
		mustAddr("192.0.2.3"),
		mustAddr("192.0.2.4"),
		mustAddr("192.0.2.5"),
	}
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		out := orderAddrs(in, PolicyRandom, false, "h", &hostCounters{})
		require.Len(t, out, len(in))
		seen[out[0].String()] = true
	}
	assert.Greater(t, len(seen), 1)
}
