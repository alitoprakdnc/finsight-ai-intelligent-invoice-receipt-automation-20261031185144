package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"go.uber.org/zap"

	"backend/internal/config"
)

// DB wraps a sql.DB instance and provides helper methods.
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// New creates a new database connection pool, runs migrations and returns a *DB.
// It expects a fully populated configuration object.
func New(cfg *config.Config, logger *zap.Logger) (*DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Initialise the connection pool.
	db, err := sql.Open("postgres", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}

	// Apply pool settings – use sensible defaults if the config values are zero.
	if cfg.Database.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}
	if cfg.Database.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	if cfg.Database.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	// Verify connectivity.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	logger.Info("PostgreSQL connection established")

	// Run migrations.
	if err := runMigrations(db, cfg, logger); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	logger.Info("Database migrations applied successfully")

	return &DB{DB: db, logger: logger}, nil
}

// runMigrations executes all up migrations located in cfg.Database.MigrationsPath.
// It uses golang‑migrate with the file source driver.
func runMigrations(db *sql.DB, cfg *config.Config, logger *zap.Logger) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migrate driver: %w", err)
	}

	migrationsPath := cfg.Database.MigrationsPath
	if migrationsPath == "" {
		// Default to the embedded migrations directory.
		migrationsPath = "file://backend/internal/db/migrations"
	}
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}
	// Use zap for migrate logs if needed.
	m.Log = &zapMigrateLogger{logger: logger}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

// Close shuts down the underlying sql.DB connection pool.
func (d *DB) Close() error {
	if d.DB == nil {
		return nil
	}
	if err := d.DB.Close(); err != nil {
		return fmt.Errorf("closing db: %w", err)
	}
	d.logger.Info("PostgreSQL connection pool closed")
	return nil
}

// Ping checks the health of the database connection.
func (d *DB) Ping(ctx context.Context) error {
	if d.DB == nil {
		return fmt.Errorf("db is nil")
	}
	return d.DB.PingContext(ctx)
}

// zapMigrateLogger adapts zap.Logger to the migrate.Logger interface.
type zapMigrateLogger struct {
	logger *zap.Logger
}

func (z *zapMigrateLogger) Printf(format string, v ...interface{}) {
	z.logger.Sugar().Infof(format, v...)
}
func (z *zapMigrateLogger) Verbose() bool { return true }