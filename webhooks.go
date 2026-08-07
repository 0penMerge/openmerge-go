package openmerge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type VerifiedWebhook struct {
	Timestamp int64
	Payload   []byte
}

func VerifyWebhookSignature(payload []byte, signatureHeader, secret string, tolerance time.Duration, now time.Time) (VerifiedWebhook, error) {
	if !strings.HasPrefix(secret, "whsec_") || len(secret) <= len("whsec_") {
		return VerifiedWebhook{}, errors.New("openmerge: webhook secret must start with whsec_")
	}
	if tolerance < 0 {
		return VerifiedWebhook{}, errors.New("openmerge: webhook tolerance must be non-negative")
	}
	var timestamp int64
	var signatures []string
	for _, part := range strings.Split(signatureHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp, _ = strconv.ParseInt(strings.TrimPrefix(part, "t="), 10, 64)
		}
		if strings.HasPrefix(part, "v1=") {
			signatures = append(signatures, strings.TrimPrefix(part, "v1="))
		}
	}
	if timestamp <= 0 || len(signatures) == 0 {
		return VerifiedWebhook{}, errors.New("openmerge: invalid webhook signature header")
	}
	if now.IsZero() {
		now = time.Now()
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > tolerance {
		return VerifiedWebhook{}, errors.New("openmerge: webhook timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, value := range signatures {
		actual, err := hex.DecodeString(value)
		if err == nil && hmac.Equal(actual, expected) {
			return VerifiedWebhook{Timestamp: timestamp, Payload: payload}, nil
		}
	}
	return VerifiedWebhook{}, errors.New("openmerge: webhook signature mismatch")
}
