package middleware

import (
	_ "embed"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

//go:embed token_bucket.lua
var tokenBucketScriptSrc string

const (
	algoFixedWindow   = "fixed_window"
	algoTokenBucket   = "token_bucket"
	algoSlidingWindow = "sliding_window"
)

var (
	rlCfg              *config.RateLimitConfig
	tokenBucketScript  = redis.NewScript(tokenBucketScriptSrc)
)

func InitRateLimit(cfg *config.RateLimitConfig) {
	rlCfg = cfg
}

func GlobalRateLimit() gin.HandlerFunc {
	cfg := rlCfg
	if cfg == nil || !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return rateLimit(cfg.DefaultRPM, cfg.Algorithm)
}

func RouteRateLimit(rpm int) gin.HandlerFunc {
	cfg := rlCfg
	algo := cfg.Algorithm
	if algo == "" {
		algo = algoFixedWindow
	}
	return rateLimit(rpm, algo)
}

func rateLimit(rpm int, algo string) gin.HandlerFunc {
	cfg := rlCfg
	if rpm <= 0 {
		rpm = 60
	}

	var limiter func(ctx *gin.Context, userID uint) (bool, int)
	switch algo {
	case algoTokenBucket:
		limiter = tokenBucketLimiter(rpm)
	case algoSlidingWindow:
		limiter = slidingWindowLimiter(rpm)
	case algoFixedWindow, "":
		limiter = fixedWindowLimiter(rpm)
	default:
		slog.Warn("unknown rate limit algorithm, fallback to fixed_window", "algo", algo)
		limiter = fixedWindowLimiter(rpm)
	}

	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		if cfg.IsWhitelisted(userID) {
			c.Next()
			return
		}
		ok, retryAfter := limiter(c, userID)
		if !ok {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}
		c.Next()
	}
}

func fixedWindowLimiter(rpm int) func(*gin.Context, uint) (bool, int) {
	return func(c *gin.Context, userID uint) (bool, int) {
		key := fmt.Sprintf("ratelimit:fw:%d", userID)
		ctx := c.Request.Context()

		pipe := database.RDB.TxPipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Minute)
		_, err := pipe.Exec(ctx)
		if err != nil {
			slog.Error("rate limit fixed_window failed", "error", err)
			return true, 0
		}

		if incr.Val() > int64(rpm) {
			ttl, _ := database.RDB.TTL(ctx, key).Result()
			return false, int(ttl.Seconds())
		}
		return true, 0
	}
}

func tokenBucketLimiter(rpm int) func(*gin.Context, uint) (bool, int) {
	return func(c *gin.Context, userID uint) (bool, int) {
		key := fmt.Sprintf("ratelimit:tb:%d", userID)
		tsKey := key + ":ts"
		ctx := c.Request.Context()

		ret, err := tokenBucketScript.Run(ctx, database.RDB,
			[]string{key, tsKey},
			time.Now().UnixMilli(), rpm, 60000).Int()
		if err != nil {
			slog.Error("rate limit token_bucket failed", "error", err)
			return true, 0
		}

		if ret != 1 {
			ttl, _ := database.RDB.TTL(ctx, key).Result()
			return false, max(int(ttl.Seconds()), 1)
		}
		return true, 0
	}
}

func slidingWindowLimiter(rpm int) func(*gin.Context, uint) (bool, int) {
	return func(c *gin.Context, userID uint) (bool, int) {
		key := fmt.Sprintf("ratelimit:sw:%d", userID)
		ctx := c.Request.Context()
		now := time.Now().UnixMilli()
		cutoff := now - time.Minute.Milliseconds()

		pipe := database.RDB.TxPipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff))
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d", now)})
		zcard := pipe.ZCard(ctx, key)
		pipe.Expire(ctx, key, time.Minute)
		_, err := pipe.Exec(ctx)
		if err != nil {
			slog.Error("rate limit sliding_window failed", "error", err)
			return true, 0
		}

		if zcard.Val() > int64(rpm) {
			ttl, _ := database.RDB.TTL(ctx, key).Result()
			return false, int(ttl.Seconds())
		}
		return true, 0
	}
}
