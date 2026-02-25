package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	// No password by default for local development

	redisDB := 0 // Default DB

	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	log.Printf("[Redis] Initializing Redis client: addr=%s, db=%d", addr, redisDB)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     redisPassword,
		DB:           redisDB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		log.Printf("[Redis] ERROR: Failed to ping Redis: %v", err)
		return client // Still return the client, connection pool will try to reconnect
	}

	log.Printf("[Redis] Successfully connected to Redis")
	return client

}
