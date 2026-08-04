package dnscache

import (
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
)

// hostCounters tracks per-host dial offsets for round-robin / rotation.
type hostCounters struct {
	m sync.Map // string -> *atomic.Uint64
}

func (h *hostCounters) next(host string) uint64 {
	v, _ := h.m.LoadOrStore(host, &atomic.Uint64{})
	return v.(*atomic.Uint64).Add(1) - 1
}

// orderAddrs applies the dial policy to a list of addresses.
func orderAddrs(addrs []netip.Addr, policy Policy, rotate bool, host string, counters *hostCounters) []netip.Addr {
	if len(addrs) <= 1 {
		return addrs
	}
	out := cloneAddrs(addrs)
	switch policy {
	case PolicyRandom:
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	case PolicyRoundRobin:
		off := counters.next(host)
		out = rotateSlice(out, int(off%uint64(len(out))))
	case PolicyInterleave:
		out = interleaveFamilies(out, rotate, host, counters)
	case PolicyFirst:
		// keep resolver order
	default:
		// unknown policy: keep order
	}
	return out
}

// interleaveFamilies splits by lead family, optionally rotates each half with a
// shared offset, then merges lead, other, lead, other, ...
func interleaveFamilies(addrs []netip.Addr, rotate bool, host string, counters *hostCounters) []netip.Addr {
	leadIs6 := addrs[0].Is6()
	var first, second []netip.Addr
	for _, a := range addrs {
		if a.Is6() == leadIs6 {
			first = append(first, a)
		} else {
			second = append(second, a)
		}
	}
	if rotate && counters != nil {
		off := counters.next(host)
		if len(first) > 0 {
			first = rotateSlice(first, int(off%uint64(len(first))))
		}
		if len(second) > 0 {
			second = rotateSlice(second, int(off%uint64(len(second))))
		}
	}
	out := make([]netip.Addr, 0, len(addrs))
	i, j := 0, 0
	for i < len(first) || j < len(second) {
		if i < len(first) {
			out = append(out, first[i])
			i++
		}
		if j < len(second) {
			out = append(out, second[j])
			j++
		}
	}
	return out
}

func rotateSlice(addrs []netip.Addr, offset int) []netip.Addr {
	n := len(addrs)
	if n == 0 {
		return addrs
	}
	offset %= n
	if offset < 0 {
		offset += n
	}
	if offset == 0 {
		return addrs
	}
	out := make([]netip.Addr, n)
	copy(out, addrs[offset:])
	copy(out[n-offset:], addrs[:offset])
	return out
}

// filterByNetwork keeps addresses matching the dial network family.
// network is the dial network (tcp, tcp4, tcp6, udp, ...).
func filterByNetwork(addrs []netip.Addr, network string) []netip.Addr {
	want4, want6 := networkFamily(network)
	if want4 && want6 {
		return addrs
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		a = a.Unmap()
		if want4 && a.Is4() {
			out = append(out, a)
		}
		if want6 && a.Is6() {
			out = append(out, a)
		}
	}
	return out
}

func networkFamily(network string) (want4, want6 bool) {
	switch network {
	case "tcp4", "udp4", "ip4":
		return true, false
	case "tcp6", "udp6", "ip6":
		return false, true
	default:
		return true, true
	}
}

// dedupeAddrs canonicalizes and removes duplicate addresses (incl. IPv4-mapped).
func dedupeAddrs(addrs []netip.Addr) []netip.Addr {
	if len(addrs) == 0 {
		return nil
	}
	seen := make(map[netip.Addr]struct{}, len(addrs))
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		a = a.Unmap()
		if !a.IsValid() {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}
