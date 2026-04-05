package helpers

import (
	"sync"
	"time"
)

// ──────────────────────────────────────────────
//  In-memory TTL cache for backend API responses
// ──────────────────────────────────────────────
//
//  Usage:
//    val, ok := APICache.Get("suppliers")
//    if !ok {
//        val = fetchFromBackend(...)
//        APICache.Set("suppliers", val, 2*time.Minute)
//    }

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// Cache is a simple thread-safe in-memory cache with per-key TTL.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// NewCache creates a new cache and starts a background cleanup goroutine.
func NewCache() *Cache {
	c := &Cache{entries: make(map[string]cacheEntry)}
	go c.janitor()
	return c
}

// Get retrieves a value. Returns (value, true) if found and not expired.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

// Set stores a value with the given TTL.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Delete removes a specific key (e.g. after a write/mutation).
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// DeletePrefix removes all keys starting with the given prefix.
func (c *Cache) DeletePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

// Flush clears the entire cache.
func (c *Cache) Flush() {
	c.mu.Lock()
	c.entries = make(map[string]cacheEntry)
	c.mu.Unlock()
}

// janitor periodically removes expired entries.
func (c *Cache) janitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, v := range c.entries {
			if now.After(v.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

// APICache is the global cache instance for backend API responses.
var APICache = NewCache()

// Cache TTL constants — tuned per data volatility
const (
	CacheTTLStores    = 5 * time.Minute  // stores rarely change
	CacheTTLBranches  = 5 * time.Minute  // branches rarely change
	CacheTTLSuppliers = 3 * time.Minute  // suppliers rarely change
	CacheTTLProducts  = 2 * time.Minute  // products change more often
	CacheTTLClients   = 2 * time.Minute  // clients change somewhat
	CacheTTLOrders    = 1 * time.Minute  // orders are active
	CacheTTLInvoices  = 30 * time.Second // invoices are most volatile
	CacheTTLPurchBill = 1 * time.Minute  // purchase bills change moderately
)
