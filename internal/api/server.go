package api

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"snmp-mqtt-bridge/internal/api/handler"
	"snmp-mqtt-bridge/internal/config"
	"snmp-mqtt-bridge/internal/mqtt"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	router     *gin.Engine
	httpServer *http.Server
	services   *Services
}

// Services contains all service dependencies
type Services struct {
	Device     *service.DeviceService
	Profile    *service.ProfileService
	TrapLog    *service.TrapLogService
	Setting    *service.SettingService
	Poller     *service.PollerService
	SNMP       *service.SNMPService
	MQTTClient *mqtt.Client
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, services *Services, frontendFS embed.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggerMiddleware())

	s := &Server{
		cfg:      cfg,
		router:   router,
		services: services,
	}

	s.setupRoutes(frontendFS)
	return s
}

func (s *Server) setupRoutes(frontendFS embed.FS) {
	// Health endpoints
	health := handler.NewHealthHandler()
	s.router.GET("/health", health.Health)
	s.router.GET("/ready", health.Ready)

	// API routes
	api := s.router.Group("/api")
	{
		// Devices
		deviceHandler := handler.NewDeviceHandler(s.services.Device, s.services.Poller)
		devices := api.Group("/devices")
		{
			devices.GET("", deviceHandler.List)
			devices.POST("", deviceHandler.Create)
			devices.GET("/:id", deviceHandler.Get)
			devices.PUT("/:id", deviceHandler.Update)
			devices.DELETE("/:id", deviceHandler.Delete)
			devices.POST("/:id/test", deviceHandler.TestConnection)
			devices.GET("/:id/state", deviceHandler.GetState)
		}
		api.POST("/test-connection", deviceHandler.TestNewConnection)

		// Profiles
		profileHandler := handler.NewProfileHandler(s.services.Profile)
		profiles := api.Group("/profiles")
		{
			profiles.GET("", profileHandler.List)
			profiles.GET("/:id", profileHandler.Get)
			profiles.POST("", profileHandler.Create)
			profiles.PUT("/:id", profileHandler.Update)
			profiles.DELETE("/:id", profileHandler.Delete)
		}

		// Traps
		trapHandler := handler.NewTrapHandler(s.services.TrapLog)
		traps := api.Group("/traps")
		{
			traps.GET("", trapHandler.List)
			traps.GET("/:id", trapHandler.Get)
			traps.DELETE("/cleanup", trapHandler.Cleanup)
		}

		// Settings
		settingHandler := handler.NewSettingHandler(s.services.Setting)
		if s.services.MQTTClient != nil {
			settingHandler.SetMQTTClient(s.services.MQTTClient)
		}
		settings := api.Group("/settings")
		{
			settings.GET("", settingHandler.List)
			settings.GET("/:key", settingHandler.Get)
			settings.PUT("/:key", settingHandler.Set)
			settings.DELETE("/:key", settingHandler.Delete)
		}

		// MQTT management
		api.GET("/mqtt/status", settingHandler.GetMQTTStatus)
		api.POST("/mqtt/reconnect", settingHandler.ReconnectMQTT)
		api.POST("/mqtt/test", settingHandler.TestMQTTConnection)

		// WebSocket for real-time updates
		wsHandler := handler.NewWebSocketHandler(s.services.Poller)
		api.GET("/ws", wsHandler.HandleWebSocket)

		// Device commands (SNMP SET)
		if s.services.SNMP != nil {
			commandHandler := handler.NewCommandHandler(s.services.SNMP, s.services.Poller)
			devices.POST("/:id/set", commandHandler.SetValue)
			devices.GET("/:id/get", commandHandler.GetValue)
			// ATS commands
			devices.POST("/:id/switch-source", commandHandler.SwitchSource)
			devices.POST("/:id/set-source-name", commandHandler.SetSourceName)
			// PDU commands
			devices.POST("/:id/outlet/state", commandHandler.SetOutletState)
			devices.POST("/:id/outlet/name", commandHandler.SetOutletName)
			devices.POST("/:id/outlet/reboot", commandHandler.RebootOutlet)
		}
	}

	// Serve embedded frontend
	s.serveFrontend(frontendFS)
}

func (s *Server) serveFrontend(frontendFS embed.FS) {
	// Try to get frontend dist directory from embed
	distFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		// Frontend not embedded, serve placeholder
		s.router.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "SNMP-MQTT Bridge API",
				"docs":    "/api",
			})
		})
		return
	}

	staticServer := http.FileServer(http.FS(distFS))

	// Serve static files
	s.router.NoRoute(func(c *gin.Context) {
		// Try to serve the file
		path := c.Request.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		if _, err := fs.Stat(distFS, path[1:]); err == nil {
			staticServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Fallback to index.html for SPA routing
		c.Request.URL.Path = "/"
		staticServer.ServeHTTP(c.Writer, c.Request)
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		if c.Request.URL.Path != "/health" && c.Request.URL.Path != "/ready" {
			fmt.Printf("[%s] %s %s %d %v\n",
				time.Now().Format(time.RFC3339),
				c.Request.Method,
				c.Request.URL.Path,
				c.Writer.Status(),
				latency,
			)
		}
	}
}
