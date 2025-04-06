package infrastructure

import (
	"context"

	"github.com/ideagate/server-proxy/config"
	"github.com/redis/go-redis/v9"
)

type Infrastructure struct {
	Redis redis.UniversalClient
}

func NewInfrastructure(ctx context.Context, cfg *config.Config) (*Infrastructure, error) {
	// Initialize redis
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    cfg.Redis.Addrs,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Infrastructure{
		Redis: redisClient,
	}, nil
}
