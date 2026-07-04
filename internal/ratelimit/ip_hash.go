package ratelimit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func HashIP(ip, secret string) string {
	ip = strings.TrimSpace(ip)
	secret = strings.TrimSpace(secret)
	if ip == "" || secret == "" {
		return ""
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}
