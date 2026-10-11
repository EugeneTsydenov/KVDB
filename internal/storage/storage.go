package storage

// Setter is used to save or update keys and values.
type Setter interface {
	// Set saves a value by its key.
	// If the key is already there, it overwrites the old value.
	Set(key string, val Value)

	// SetNX saves a value only if the key does not exist yet
	// or if the old value has expired.
	// Returns true if it successfully saved the value, or false if it did not.
	SetNX(key string, val Value) bool
}

// Getter is used to read values from the store.
type Getter interface {
	// Get finds a value by its key.
	// Returns ErrEntryNotFound if the key is missing or has expired.
	// Expired keys are deleted automatically when you try to read them.
	Get(key string) (Value, error)
}

// Deleter is used to remove keys from the store.
type Deleter interface {
	// Del deletes a key and its value.
	// Returns true if the key existed and was deleted, or false if the key was not found.
	Del(key string) bool
}

// Storage combines Setter, Getter, and Deleter into one main interface.
// All implementations must be safe for concurrent use by multiple routines.
type Storage interface {
	Setter
	Getter
	Deleter
}
