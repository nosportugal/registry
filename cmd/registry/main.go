package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/registry/internal/api"
	v0 "github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/importer"
	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/storage"
	"github.com/modelcontextprotocol/registry/internal/telemetry"
)

// Version info for the MCP Registry application
// These variables are injected at build time via ldflags
var (
	// Version is the current version of the MCP Registry application
	Version = "dev"

	// BuildTime is the time at which the binary was built
	BuildTime = "unknown"

	// GitCommit is the git commit that was compiled
	GitCommit = "unknown"
)

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Display version information")
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		log.Printf("MCP Registry %s\n", Version)
		log.Printf("Git commit: %s\n", GitCommit)
		log.Printf("Build time: %s\n", BuildTime)
		return
	}

	log.Printf("Starting MCP Registry Application v%s (commit: %s)", Version, GitCommit)

	// Initialize configuration
	cfg := config.NewConfig()

	// Initialize in-memory storage
	st := storage.NewInMemoryStorage()
	log.Println("Initialized in-memory storage")

	// Create registry service with in-memory storage
	registryService := service.NewRegistryService(st, cfg)

	// Import seed data if seed source is provided
	if cfg.SeedFrom != "" {
		log.Printf("Importing data from %s...", cfg.SeedFrom)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		importerService := importer.NewService(registryService, st)
		if err := importerService.ImportFromPath(ctx, cfg.SeedFrom); err != nil {
			log.Printf("Failed to import seed data: %v", err)
		}
	}

	shutdownTelemetry, metrics, err := telemetry.InitMetrics(cfg.Version)
	if err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
		return
	}

	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Printf("Failed to shutdown telemetry: %v", err)
		}
	}()

	// Prepare version information
	versionInfo := &v0.VersionBody{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
	}

	// Initialize HTTP server
	server := api.NewServer(cfg, registryService, metrics, versionInfo)

	// Start server in a goroutine so it doesn't block signal handling
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()

	// Gracefully shutdown the server
	if err := server.Shutdown(sctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
