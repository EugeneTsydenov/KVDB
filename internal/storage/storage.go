package storage

import "time"

// Setter stores or updates key-value pairs.
type Setter interface {
	// Set stores a value by key, overwriting any existing value
	Set(key string, v Value)

	// SetNX stores a value only if the key does not exist or has expired
	// Returns true if the value was saved, or false otherwise
	SetNX(key string, v Value) bool

	// SetEX stores a value with a ttl duration, overwriting any existing value
	// Returns ErrTTLInvalod if ttl <= 0
	SetEX(key string, v Value, ttl time.Duration) error
}

// Getter reads values from the store.
type Getter interface {
	// Get retrieves a value by key
	// Returns ErrEntryNotFound if the key does not exist or has expired
	Get(key string) (Value, error)
}

// Deleter removes keys from the store.
type Deleter interface {
	// Del removes a key and its value
	// Returns true if the key was deleted, or false if it was not found
	Del(key string) bool
}

// Expirer manages key expiration and ttls.
type Expirer interface {
	// Expire sets or updates ttl for an existing key
	// Returns ErrEntryNotFound if the key does not exist or has expired,
	// or ErrTTLInvalid if ttl <= 0
	Expire(key string, ttl time.Duration) error

	// TTL returns the remaining ttl for a key
	// Returns ErrEntryNotFound if the key does not exist or has expired
	// or ErrKeyNoTTL if the key exists withoud ttl
	TTL(key string) (time.Duration, error)
}

// Storage combines Setter, Getter, Deleter, and Expirer into a single interface
// All implementations must be safe for concurrent use by multiple goroutines.
type Storage interface {
	Setter
	Getter
	Deleter
	Expirer
}
