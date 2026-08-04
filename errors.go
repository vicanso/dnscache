package dnscache

import "errors"

var (
	// ErrNotFound indicates the host is absent from storage or has no usable addresses.
	ErrNotFound = errors.New("dnscache: not found")

	// ErrNoAddresses indicates no addresses remained after network-family filtering.
	ErrNoAddresses = errors.New("dnscache: no addresses")

	// ErrClosed indicates the cache has been closed.
	ErrClosed = errors.New("dnscache: closed")

	// ErrInvalidConfig indicates New rejected the configuration.
	ErrInvalidConfig = errors.New("dnscache: invalid config")

	// ErrNegativeCache indicates a cached negative (failed) lookup is still fresh.
	ErrNegativeCache = errors.New("dnscache: negative cache hit")
)
