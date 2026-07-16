// Package cache is a small persistent key-value store backed by BadgerDB. It
// holds analysis results between runs so that unchanged inputs do not need to be
// recomputed. Values are encoded with msgpack. Keys are namespaced and carry a
// schema version, so a format change quietly invalidates old entries. Read and
// decode failures are treated as cache misses, never as errors, so a damaged
// cache degrades to a slower run rather than a broken one.
package cache

import (
	"os"
	"path/filepath"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// schemaVersion prefixes every key. Bump it when a stored value's shape changes.
const schemaVersion = "v1"

// Cache is a handle to the on-disk store.
type Cache struct {
	db *badger.DB
}

// nopLogger silences BadgerDB's internal logging.
type nopLogger struct{}

func (nopLogger) Errorf(string, ...any)   {}
func (nopLogger) Warningf(string, ...any) {}
func (nopLogger) Infof(string, ...any)    {}
func (nopLogger) Debugf(string, ...any)   {}

// Open opens or creates a cache at dir.
func Open(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(dir).
		WithLogger(nopLogger{}).
		WithNumVersionsToKeep(1)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Cache{db: db}, nil
}

// OpenOrReset opens the cache and, if opening fails because the directory is
// damaged, removes it and tries once more. This keeps a corrupted cache from
// blocking the tool.
func OpenOrReset(dir string) (*Cache, error) {
	c, err := Open(dir)
	if err == nil {
		return c, nil
	}
	if rmErr := os.RemoveAll(dir); rmErr != nil {
		return nil, err
	}
	return Open(dir)
}

// DefaultDir returns the standard cache location, honoring XDG_CACHE_HOME.
func DefaultDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "gpt", "pypls"), nil
}

// Close flushes and closes the store.
func (c *Cache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func makeKey(namespace, id string) []byte {
	return []byte(schemaVersion + ":" + namespace + ":" + id)
}

// getBytes returns the raw value for a key, or false if it is absent.
func (c *Cache) getBytes(namespace, id string) ([]byte, bool) {
	var out []byte
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(makeKey(namespace, id))
		if err != nil {
			return err
		}
		out, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, false
	}
	return out, true
}

// putBytes stores a raw value.
func (c *Cache) putBytes(namespace, id string, value []byte) error {
	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Set(makeKey(namespace, id), value)
	})
}

// GetValue decodes a stored msgpack value into type T. A missing key or a decode
// failure both report a miss, so a stale or damaged entry is simply recomputed.
func GetValue[T any](c *Cache, namespace, id string) (T, bool) {
	var zero T
	if c == nil {
		return zero, false
	}
	raw, ok := c.getBytes(namespace, id)
	if !ok {
		return zero, false
	}
	var v T
	if err := msgpack.Unmarshal(raw, &v); err != nil {
		return zero, false
	}
	return v, true
}

// PutValue encodes a value with msgpack and stores it under the key.
func PutValue[T any](c *Cache, namespace, id string, value T) error {
	if c == nil {
		return nil
	}
	raw, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}
	return c.putBytes(namespace, id, raw)
}
