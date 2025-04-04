package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bayu-aditya/ideagate/backend/core/adapter/redis"
	"github.com/bayu-aditya/ideagate/backend/core/utils/log"
	"github.com/bayu-aditya/ideagate/backend/server/proxy/config"
	"github.com/bayu-aditya/ideagate/backend/server/proxy/infrastructure"
	"github.com/bayu-aditya/ideagate/backend/server/proxy/usecase"
)

func NewServer(configFileName string) {
	ctx := context.Background()

	// Initialize config
	cfg, err := config.NewConfig(configFileName)
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	// Initialize infrastructure
	infra, err := infrastructure.NewInfrastructure(ctx, cfg)
	if err != nil {
		log.Fatal("Failed to initialize infrastructure: %v", err)
	}

	// Initialize adapter
	redisAdapter := redis.NewRedisAdapter(infra.Redis)
	pubSubAdapter := redisAdapter
	distributedLockAdapter := redisAdapter

	// Initialize usecase
	usecaseWebsocketClientManagement := usecase.NewWebsocketClientManagement(pubSubAdapter, distributedLockAdapter)

	// Run subscriber worker
	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error("Recovered in RunSubscriber: %v", r)
					}
				}()
				RunSubscriber(pubSubAdapter, usecaseWebsocketClientManagement)
			}()
		}
	}()

	// Run REST HTTP server
	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error("Recovered in RunRestHttp: %v", r)
					}
				}()
				RunRestHttp(pubSubAdapter, usecaseWebsocketClientManagement)
			}()
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Info("Shutting down proxy...")
}
