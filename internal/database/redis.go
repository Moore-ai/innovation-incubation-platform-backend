package database

import (
	"log/slog"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func NewRedisClient(addr, password string, db int) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	RDB = rdb
	slog.Info("redis connected", "addr", addr)
	return rdb
}
