// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cache implements an caches for objects.
package cache

import (
	"context"
	"fmt"
	"hash"
	"io"
	"time"
)

var (
	ErrMissingFetchFunc = fmt.Errorf("missing fetch function")
	ErrNotFound         = fmt.Errorf("key not found")
	ErrStopped          = fmt.Errorf("cacher is stopped")
)

// FetchFunc is a function used to Fetch in a cacher.
type FetchFunc func() (interface{}, error)

// KeyFunc is a function that mutates the provided cache key before storing it
// in Redis. This can be used to hash or HMAC values to prevent their plaintext
// from appearing in Redis. A good example might be an API key lookup that you
// HMAC before passing along.
//
// The KeyFunc can also be used to add a prefix or namespace to keys in
// multi-tenant systems.
type KeyFunc func(string) (string, error)

// MultiKeyFunc returns a KeyFunc that calls the provided KeyFuncs in order,
// with the previous value passed to the next.
func MultiKeyFunc(fns ...KeyFunc) KeyFunc {
	return func(in string) (string, error) {
		var err error
		for _, fn := range fns {
			in, err = fn(in)
			if err != nil {
				return "", err
			}
		}

		return in, nil
	}
}

// PrefixKeyFunc returns a KeyFunc that prefixes the key with the given constant
// before passing it to the cacher for storage.
func PrefixKeyFunc(prefix string) KeyFunc {
	return func(in string) (string, error) {
		if prefix != "" {
			in = prefix + in
		}
		return in, nil
	}
}

// HashKeyFunc returns a KeyFunc that hashes or HMACs the provided key before
// passing it to the cacher for storage.
func HashKeyFunc(hasher func() hash.Hash) KeyFunc {
	return func(in string) (string, error) {
		h := hasher()
		n, err := h.Write([]byte(in))
		if err != nil {
			return "", err
		}
		if got, want := n, len(in); got < want {
			return "", fmt.Errorf("only hashed %d of %d bytes", got, want)
		}
		dig := h.Sum(nil)
		return fmt.Sprintf("%x", dig), nil
	}
}

// Cacher is an interface that defines caching.
type Cacher interface {
	// Closer closes the cache, cleaning up any stale entries. A closed cache is
	// no longer valid and attempts to call methods should return an error or
	// (less preferred) panic.
	io.Closer

	// Fetch retrieves the named item from the cache. If the item does not exist,
	// it calls FetchFunc to create the item. If FetchFunc returns an error, the
	// error is bubbled up the stack and no value is cached. If FetchFunc
	// succeeds, the value is cached for the provided TTL.
	Fetch(context.Context, string, interface{}, time.Duration, FetchFunc) error

	// Read gets an item from the cache and reads it into the provided interface.
	// If it does not exist, it returns ErrNotFound.
	Read(context.Context, string, interface{}) error

	// Write adds an item to the cache, overwriting if it already exists, caching
	// for TTL. It returns any errors that occur on writing.
	Write(context.Context, string, interface{}, time.Duration) error

	// Delete removes an item from the cache, returning any errors that occur.
	Delete(context.Context, string) error
}
