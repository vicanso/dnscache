package dnscache

import (
	"net/netip"
	"time"
)

// Entry is a cached DNS result.
//
// A permanent entry has FreshUntil and StaleUntil both zero and Negative false.
// A negative entry has Negative true and empty Addrs; it is fresh until FreshUntil.
type Entry struct {
	Addrs      []netip.Addr
	FreshUntil time.Time
	StaleUntil time.Time
	Negative   bool
}

// Permanent returns a never-expiring positive entry.
func Permanent(addrs ...netip.Addr) Entry {
	return Entry{Addrs: append([]netip.Addr(nil), addrs...)}
}

// IsPermanent reports whether the entry never expires by time.
func (e Entry) IsPermanent() bool {
	return !e.Negative && e.FreshUntil.IsZero() && e.StaleUntil.IsZero()
}

// IsFresh reports whether the entry may be served without refresh.
func (e Entry) IsFresh(now time.Time) bool {
	if e.Negative {
		return !e.FreshUntil.IsZero() && now.Before(e.FreshUntil)
	}
	if e.IsPermanent() {
		return len(e.Addrs) > 0
	}
	return now.Before(e.FreshUntil)
}

// IsStale reports whether the entry is past fresh but still within the SWR window.
func (e Entry) IsStale(now time.Time) bool {
	if e.Negative || e.IsPermanent() || len(e.Addrs) == 0 {
		return false
	}
	if now.Before(e.FreshUntil) {
		return false
	}
	return !e.StaleUntil.IsZero() && now.Before(e.StaleUntil)
}

// IsExpired reports whether the entry must not be served.
func (e Entry) IsExpired(now time.Time) bool {
	if e.IsFresh(now) || e.IsStale(now) {
		return false
	}
	return true
}

// cloneAddrs returns a defensive copy of addresses.
func cloneAddrs(addrs []netip.Addr) []netip.Addr {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]netip.Addr, len(addrs))
	copy(out, addrs)
	return out
}

// cloneEntry returns a deep copy of the entry.
func cloneEntry(e Entry) Entry {
	return Entry{
		Addrs:      cloneAddrs(e.Addrs),
		FreshUntil: e.FreshUntil,
		StaleUntil: e.StaleUntil,
		Negative:   e.Negative,
	}
}
