package dnscache

import (
	"net/netip"
	"strings"
)

// normalizeHost prepares a cache key from a hostname or IP literal.
//
// Rules (RFC 4343 ASCII case-insensitivity):
//   - strip surrounding [] used for IPv6 literals in host:port forms
//   - strip a single trailing root dot (except the root name ".")
//   - fold only ASCII A-Z to a-z (no Unicode case mapping)
func normalizeHost(host string) string {
	if host == "" {
		return host
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) > 2 {
		host = host[1 : len(host)-1]
	}
	if host != "." && strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
	return asciiLower(host)
}

// asciiLower folds ASCII uppercase bytes only; leaves all other bytes intact.
func asciiLower(host string) string {
	var b []byte
	for i := 0; i < len(host); i++ {
		c := host[i]
		if c < 'A' || c > 'Z' {
			continue
		}
		if b == nil {
			b = []byte(host)
		}
		b[i] = c + ('a' - 'A')
	}
	if b == nil {
		return host
	}
	return string(b)
}

// parseIPLiteral returns a netip.Addr when host is an IP literal.
func parseIPLiteral(host string) (netip.Addr, bool) {
	host = normalizeHost(host)
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}
