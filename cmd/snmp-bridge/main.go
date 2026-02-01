package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"snmp-mqtt-bridge/internal/api"
	"snmp-mqtt-bridge/internal/config"
	"snmp-mqtt-bridge/internal/domain"
	embedfs "snmp-mqtt-bridge/internal/embed"
	"snmp-mqtt-bridge/internal/mqtt"
	"snmp-mqtt-bridge/internal/repository/sqlite"
	"snmp-mqtt-bridge/internal/service"
	"snmp-mqtt-bridge/internal/worker"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting SNMP-MQTT Bridge...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := sqlite.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create repositories
	deviceRepo := sqlite.NewDeviceRepository(db)
	profileRepo := sqlite.NewProfileRepository(db)
	trapRepo := sqlite.NewTrapLogRepository(db)
	settingRepo := sqlite.NewSettingRepository(db)

	// Create services
	deviceService := service.NewDeviceService(deviceRepo)
	profileService := service.NewProfileService(profileRepo)
	trapLogService := service.NewTrapLogService(trapRepo)
	settingService := service.NewSettingService(settingRepo)

	// Load built-in profiles
	if err := profileService.LoadBuiltinProfiles(context.Background(), "profiles"); err != nil {
		log.Printf("Warning: Failed to load built-in profiles: %v", err)
	}

	// Create poller service
	pollerService := service.NewPollerService(deviceRepo, profileRepo, cfg.SNMP.PollInterval)

	// Create SNMP service for commands
	snmpService := service.NewSNMPService(deviceRepo, profileRepo)

	// Create MQTT client
	mqttClient := mqtt.NewClient(&cfg.MQTT)
	if err := mqttClient.Connect(); err != nil {
		log.Printf("Warning: Failed to connect to MQTT broker: %v", err)
	}

	// Create MQTT discovery and publisher
	discovery := mqtt.NewDiscovery(mqttClient, cfg.MQTT.DiscoveryPrefix, cfg.MQTT.TopicPrefix)
	publisher := mqtt.NewPublisher(mqttClient, discovery, pollerService, profileRepo)

	// Create trap receiver
	trapReceiver := worker.NewTrapReceiver(cfg.SNMP.TrapPort, deviceRepo, trapRepo, pollerService)

	// Trap event handler - publish to MQTT
	trapReceiver.OnTrap(func(trapLog *domain.TrapLog) {
		if mqttClient.IsConnected() {
			topic := fmt.Sprintf("%s/traps", cfg.MQTT.TopicPrefix)
			mqttClient.Publish(topic, trapLog, false)
		}
	})

	// Create API server
	services := &api.Services{
		Device:     deviceService,
		Profile:    profileService,
		TrapLog:    trapLogService,
		Setting:    settingService,
		Poller:     pollerService,
		SNMP:       snmpService,
		MQTTClient: mqttClient,
	}

	server := api.NewServer(cfg, services, embedfs.FrontendFS)

	// Start services
	ctx := context.Background()

	if err := pollerService.Start(ctx); err != nil {
		log.Fatalf("Failed to start poller: %v", err)
	}

	if err := publisher.Start(); err != nil {
		log.Printf("Warning: Failed to start MQTT publisher: %v", err)
	}

	// Register existing devices with MQTT publisher
	devices, _ := deviceService.GetEnabled(ctx)
	for i := range devices {
		if err := publisher.RegisterDevice(&devices[i]); err != nil {
			log.Printf("Warning: Failed to register device %s with MQTT: %v", devices[i].ID, err)
		}
	}

	if err := trapReceiver.Start(); err != nil {
		log.Printf("Warning: Failed to start trap receiver: %v", err)
	}

	// Start HTTP server in goroutine
	go func() {
		log.Printf("HTTP server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop services in reverse order
	trapReceiver.Stop()
	publisher.Stop()
	pollerService.Stop()
	mqttClient.Disconnect()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
