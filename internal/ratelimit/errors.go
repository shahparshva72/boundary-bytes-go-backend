package ratelimit

import "errors"

var ErrDailyLimitExceeded = errors.New("daily rate limit exceeded")
