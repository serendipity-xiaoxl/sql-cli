package db

import (
	"fmt"
	"sync"

	"github.com/xiaoxl/sql-cli/pkg/config"
)

// DriverFactory creates a Session from a DSN and configuration.
// Each registered driver provides its own factory for opening
// database-specific connections.
type DriverFactory func(dsn string, cfg *config.Config) (*Session, error)

var (
	driversMu sync.RWMutex
	drivers   = map[string]DriverFactory{}
)

// RegisterDriver registers a driver factory for the given driver name.
// Typically called from init() in driver implementation packages (e.g., pkg/db/mysql).
// If a factory for the same name is already registered, it is replaced.
func RegisterDriver(name string, factory DriverFactory) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = factory
}

// Open creates a new Session using the registered driver factory for the
// given driver name. The driver must have been registered via RegisterDriver(),
// typically by importing the driver implementation package.
//
// Usage:
//
//	import _ "github.com/xiaoxl/sql-cli/pkg/db/mysql"
//	sess, err := db.Open("mysql", dsn, opts...)
func Open(driver, dsn string, options ...config.Option) (*Session, error) {
	cfg := config.DefaultConfig()
	for _, opt := range options {
		opt(cfg)
	}

	driversMu.RLock()
	factory, ok := drivers[driver]
	driversMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown database driver: %q (forgot to import the driver package?)", driver)
	}

	return factory(dsn, cfg)
}
