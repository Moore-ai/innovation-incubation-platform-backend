package database

import (
	"context"
	"log/slog"
	"os"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func MustNewRedisClient(addr, password string, db int) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	RDB = rdb
	slog.Info("redis connected", "addr", addr)
	return rdb
}
