package dnscache

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorageLRU(t *testing.T) {
	s := NewMemoryStorage(2)
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "a", Permanent(mustAddr("192.0.2.1"))))
	require.NoError(t, s.Set(ctx, "b", Permanent(mustAddr("192.0.2.2"))))
	// touch a so b is older
	_, err := s.Get(ctx, "a")
	require.NoError(t, err)
	require.NoError(t, s.Set(ctx, "c", Permanent(mustAddr("192.0.2.3"))))

	_, err = s.Get(ctx, "b")
	assert.ErrorIs(t, err, ErrNotFound)
	e, err := s.Get(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.1", e.Addrs[0].String())
	e, err = s.Get(ctx, "c")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.3", e.Addrs[0].String())
	assert.Equal(t, 2, s.Len())
}

func TestMemoryStorageUpdateAndDelete(t *testing.T) {
	s := NewMemoryStorage(10)
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, "h", Permanent(mustAddr("192.0.2.1"))))
	require.NoError(t, s.Set(ctx, "h", Permanent(mustAddr("192.0.2.9"))))
	e, err := s.Get(ctx, "h")
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("192.0.2.9"), e.Addrs[0])
	require.NoError(t, s.Delete(ctx, "h"))
	_, err = s.Get(ctx, "h")
	assert.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, s.Purge(ctx))
}

func TestMemoryStorageClonesEntries(t *testing.T) {
	s := NewMemoryStorage(4)
	ctx := context.Background()
	entry := Entry{
		Addrs:      []netip.Addr{mustAddr("192.0.2.1")},
		FreshUntil: time.Now().Add(time.Minute),
	}
	require.NoError(t, s.Set(ctx, "h", entry))
	entry.Addrs[0] = mustAddr("192.0.2.9")
	got, err := s.Get(ctx, "h")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.1", got.Addrs[0].String())
	got.Addrs[0] = mustAddr("198.51.100.1")
	got2, err := s.Get(ctx, "h")
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.1", got2.Addrs[0].String())
}
