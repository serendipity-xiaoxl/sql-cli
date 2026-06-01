// Package registry provides a thread-safe registry of named database sessions.
package registry

import (
	"errors"
	"fmt"
	"sync"

	"github.com/xiaoxl/sql-cli/pkg/config"
	"github.com/xiaoxl/sql-cli/pkg/db"
)

// Registry manages named database sessions.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*db.Session
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]*db.Session),
	}
}

// Open creates a new named session and stores it in the registry.
// The name is used as the session's identifier (available via Session.Name()).
// Returns an error if a session with the same name already exists.
func (r *Registry) Open(name, driver, dsn string, opts ...config.Option) (*db.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[name]; exists {
		return nil, fmt.Errorf("session %q already exists in registry", name)
	}

	// Prepend WithName so the registry name becomes the session name.
	// User-supplied WithName options are applied afterwards and override if present.
	localOpts := make([]config.Option, 0, len(opts)+1)
	localOpts = append(localOpts, config.WithName(name))
	localOpts = append(localOpts, opts...)

	session, err := db.Open(driver, dsn, localOpts...)
	if err != nil {
		return nil, err
	}

	r.sessions[name] = session
	return session, nil
}

// Get retrieves a session by name. Returns an error if not found.
func (r *Registry) Get(name string) (*db.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[name]
	if !exists {
		return nil, fmt.Errorf("session %q not found in registry", name)
	}
	return session, nil
}

// List returns the names of all registered sessions.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.sessions))
	for name := range r.sessions {
		names = append(names, name)
	}
	return names
}

// Add stores an existing session in the registry under the given name.
// Returns an error if a session with the same name already exists.
func (r *Registry) Add(name string, session *db.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[name]; exists {
		return fmt.Errorf("session %q already exists in registry", name)
	}

	r.sessions[name] = session
	return nil
}

// Close closes the named session and removes it from the registry.
// Returns an error if the session is not found or close fails.
func (r *Registry) Close(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[name]
	if !exists {
		return fmt.Errorf("session %q not found in registry", name)
	}

	delete(r.sessions, name)
	return session.Close()
}

// CloseAll gracefully closes all sessions and removes them from the registry.
// Errors from individual session closes are collected and returned as a single
// joined error. All sessions are removed from the registry regardless of errors.
func (r *Registry) CloseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, session := range r.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %q: %w", name, err))
		}
		delete(r.sessions, name)
	}

	return errors.Join(errs...)
}

// Count returns the number of registered sessions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.sessions)
}
