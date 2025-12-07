package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chew-z/copilot-proxy/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	router *gin.Engine
	server *http.Server
	client *http.Client
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, host string, port int) *Server {
	// Set Gin mode based on config
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Setup logging
	logPath := filepath.Join(os.TempDir(), "copilot-proxy.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("Could not create log file", "path", logPath, "error", err)
	} else {
		// Determine log level based on debug mode
		logLevel := slog.LevelInfo
		if cfg.Debug {
			logLevel = slog.LevelDebug
		}

		// Setup writers based on verbose mode (default: quiet, log to file only)
		if cfg.Verbose {
			// Verbose mode: log to both file and stdout
			gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)
			gin.DefaultErrorWriter = io.MultiWriter(logFile, os.Stderr)

			handler := slog.NewTextHandler(io.MultiWriter(logFile, os.Stdout), &slog.HandlerOptions{
				Level: logLevel,
			})
			slog.SetDefault(slog.New(handler))
			slog.Info("Logging initialized", "path", logPath)
		} else {
			// Quiet mode (default): log to file only
			gin.DefaultWriter = logFile
			gin.DefaultErrorWriter = logFile

			handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			})
			slog.SetDefault(slog.New(handler))
		}
	}

	if !cfg.Debug {
		// In release mode, disable console color for cleaner logs
		gin.DisableConsoleColor()
	}

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add logger middleware in debug mode
	if cfg.Debug {
		router.Use(gin.Logger())
	}

	// Add CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Create optimized HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 50, // Default is 2, way too low for concurrent requests
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 120 * time.Second,
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    getAddr(host, port),
		Handler: router,
	}

	server := &Server{
		config: cfg,
		router: router,
		server: srv,
		client: client,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// CreateShutdownContext creates a context for graceful shutdown
func CreateShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// setupRoutes sets up all the routes for the server
func (s *Server) setupRoutes() {
	// Static endpoints
	s.router.GET("/api/tags", s.handleTags)
	s.router.GET("/api/list", s.handleTags) // Alias for /api/tags
	s.router.GET("/api/version", s.handleVersion)
	s.router.GET("/api/ps", s.handlePs)
	s.router.POST("/api/show", s.handleShow)

	// Proxy endpoint
	s.router.POST("/v1/chat/completions", s.handleChatCompletions)
	s.router.POST("/api/chat", s.handleChatCompletions) // Alias for v1/chat/completions

	// Optional health check endpoint
	s.router.GET("/healthz", s.handleHealth)
}

// getAddr returns the address string from host and port
func getAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

// handleHealth is a simple health check endpoint
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
