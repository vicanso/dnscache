//go:build integration

package dnscache

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Run with: go test -tags=integration -count=1 ./...
func TestIntegrationHTTPDial(t *testing.T) {
	c, err := New(Config{
		TTL:   time.Minute,
		Stale: 30 * time.Second,
	})
	require.NoError(t, err)
	defer c.Close()

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: c.DialContext,
		},
	}
	resp, err := client.Get("https://example.com/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// second request should hit cache (no assertion on resolver; just ensure still works)
	resp2, err := client.Get("https://example.com/")
	require.NoError(t, err)
	defer resp2.Body.Close()
}

func TestIntegrationLookupHost(t *testing.T) {
	c, err := New(Config{TTL: time.Minute})
	require.NoError(t, err)
	defer c.Close()

	addrs, err := c.LookupHost(context.Background(), "example.com")
	require.NoError(t, err)
	require.NotEmpty(t, addrs)
}
