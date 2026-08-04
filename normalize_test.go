package dnscache

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{"[2001:db8::1]", "2001:db8::1"},
		{".", "."},
		{"", ""},
		{"API.Example.com.", "api.example.com"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, normalizeHost(tc.in), tc.in)
	}
}

func TestAsciiLowerDoesNotFoldUnicode(t *testing.T) {
	// Turkish dotted capital I should not become ASCII 'i' via Unicode lower.
	const dottedI = "\u0130stanbul"
	assert.Equal(t, dottedI, asciiLower(dottedI))
	assert.Equal(t, "abc", asciiLower("ABC"))
	assert.Equal(t, "api.example.com", asciiLower("API.Example.COM"))
}

func TestParseIPLiteral(t *testing.T) {
	addr, ok := parseIPLiteral("127.0.0.1")
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), addr)

	addr, ok = parseIPLiteral("[::1]")
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("::1"), addr)

	_, ok = parseIPLiteral("example.com")
	assert.False(t, ok)

	// IPv4-mapped unmapped
	addr, ok = parseIPLiteral("::ffff:192.0.2.1")
	require.True(t, ok)
	assert.True(t, addr.Is4())
	assert.Equal(t, "192.0.2.1", addr.String())
}
