// Package keylock provides one mutex per string key, for holding a lock across an entire
// read-modify-write sequence on a keyed record (get current value, compute a change, save it
// back). Without it, two goroutines writing the same key can each read the old value, compute
// independently, and save - whichever save lands second silently reverts the first, since
// neither knew about the other. Different keys never contend with each other, so a Registry
// does not serialize unrelated writes - only two writers touching the same key at the same time.
package keylock

import "sync"

// Registry hands out one mutex per key, created lazily and never removed - the number of
// distinct keys in a wiki is bounded, so leaking one small mutex per key that's ever been
// locked is an acceptable trade for avoiding reference-counted cleanup.
type Registry struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New creates an empty registry.
func New() *Registry {
	return &Registry{locks: map[string]*sync.Mutex{}}
}

// Lock acquires key's mutex and returns the func that releases it - pair with
// `defer unlock()`. Not reentrant: locking a key already held by the current goroutine
// deadlocks it, and holding one key's lock while locking another (fan-out) risks an ABBA
// deadlock against a goroutine doing the mirror-image update - release the first key's lock
// before acquiring the second's.
func (r *Registry) Lock(key string) (unlock func()) {
	r.mu.Lock()
	m, ok := r.locks[key]
	if !ok {
		m = &sync.Mutex{}
		r.locks[key] = m
	}
	r.mu.Unlock()

	m.Lock()
	return m.Unlock
}

// Mutate runs fn with key's mutex held for fn's entire call.
func (r *Registry) Mutate(key string, fn func() error) error {
	unlock := r.Lock(key)
	defer unlock()
	return fn()
}
