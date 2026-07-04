package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const DefaultDailyLimit = 20

type DailyLimiter struct {
	redis      *UpstashClient
	ipSecret   string
	dailyLimit int
}

func NewDailyLimiter(redis *UpstashClient, ipHashSecret string, dailyLimit int) (*DailyLimiter, error) {
	if redis == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if ipHashSecret == "" {
		return nil, fmt.Errorf("rate limit ip hash secret is required")
	}
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailyLimit
	}

	return &DailyLimiter{
		redis:      redis,
		ipSecret:   ipHashSecret,
		dailyLimit: dailyLimit,
	}, nil
}

func (l *DailyLimiter) Status(ctx context.Context, clientIP string) (models.RateLimitStatus, error) {
	now := time.Now().UTC()
	key, err := l.redisKey(clientIP, now)
	if err != nil {
		return models.RateLimitStatus{}, err
	}

	used, err := l.redis.GetInt(ctx, key)
	if err != nil {
		return models.RateLimitStatus{}, err
	}

	return buildRateLimitStatus(used, l.dailyLimit, now), nil
}

func (l *DailyLimiter) Consume(ctx context.Context, clientIP string) (models.RateLimitStatus, error) {
	now := time.Now().UTC()
	key, err := l.redisKey(clientIP, now)
	if err != nil {
		return models.RateLimitStatus{}, err
	}

	used, err := l.redis.Incr(ctx, key)
	if err != nil {
		return models.RateLimitStatus{}, err
	}

	if used == 1 {
		ttl := secondsUntil(endOfDayUTC(now))
		if err := l.redis.Expire(ctx, key, ttl); err != nil {
			return models.RateLimitStatus{}, err
		}
	}

	status := buildRateLimitStatus(used, l.dailyLimit, now)
	if used > l.dailyLimit {
		return status, ErrDailyLimitExceeded
	}

	return status, nil
}

func (l *DailyLimiter) redisKey(clientIP string, now time.Time) (string, error) {
	hashedIP := HashIP(clientIP, l.ipSecret)
	if hashedIP == "" {
		return "", fmt.Errorf("unable to hash client ip")
	}

	date := now.UTC().Format("2006-01-02")
	return fmt.Sprintf("bb:text-to-sql:%s:%s", date, hashedIP), nil
}

func buildRateLimitStatus(used, limit int, now time.Time) models.RateLimitStatus {
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	return models.RateLimitStatus{
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		ResetsAt:  endOfDayUTC(now),
	}
}

func startOfDayUTC(now time.Time) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func EndOfDayUTC(now time.Time) time.Time {
	return startOfDayUTC(now).Add(24 * time.Hour)
}

func endOfDayUTC(now time.Time) time.Time {
	return EndOfDayUTC(now)
}

func secondsUntil(target time.Time) int {
	ttl := int(time.Until(target).Seconds())
	if ttl < 1 {
		return 1
	}
	return ttl
}
