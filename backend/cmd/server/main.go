package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fin-sight/finsight/backend/internal/config"
	"github.com/fin-sight/finsight/backend/internal/db"
	"github.com/fin-sight/finsight/backend/internal/logger"
	"github.com/fin-sight/finsight/backend/internal/router"
	"github.com/go-redis/redis/v8"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// -------------------------------------------------------------------------
	// Load configuration
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// Initialise logger
	// -------------------------------------------------------------------------
	log := logger.New(cfg)
	defer func() {
		_ = log.Sync()
	}()

	// -------------------------------------------------------------------------
	// Set Gin mode based on environment
	// -------------------------------------------------------------------------
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// -------------------------------------------------------------------------
	// Initialise PostgreSQL connection pool
	// -------------------------------------------------------------------------
	dbConn, err := db.New(cfg)
	if err != nil {
		log.Fatal("failed to initialise database", zap.Error(err))
	}
	// Ensure DB is closed on exit
	defer func() {
		if cerr := dbConn.Close(); cerr != nil {
			log.Error("error closing database", zap.Error(cerr))
		}
	}()

	// -------------------------------------------------------------------------
	// Initialise Redis client
	// -------------------------------------------------------------------------
	redisOpts := &redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	redisClient := redis.NewClient(redisOpts)

	// Verify Redis connectivity
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("unable to connect to Redis", zap.Error(err))
	}
	// Ensure Redis is closed on exit
	defer func() {
		if cerr := redisClient.Close(); cerr != nil {
			log.Error("error closing redis client", zap.Error(cerr))
		}
	}()

	// -------------------------------------------------------------------------
	// Build HTTP router
	// -------------------------------------------------------------------------
	r := router.NewRouter(dbConn, redisClient, log, cfg)

	// -------------------------------------------------------------------------
	// Configure HTTP server
	// -------------------------------------------------------------------------
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// -------------------------------------------------------------------------
	// Run server in a separate goroutine
	// -------------------------------------------------------------------------
	go func() {
		log.Info("starting HTTP server", zap.String("address", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("unexpected server error", zap.Error(err))
		}
	}()

	// -------------------------------------------------------------------------
	// Graceful shutdown handling
	// -------------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // block until signal is received

	log.Info("shutdown signal received, commencing graceful shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped gracefully")
}