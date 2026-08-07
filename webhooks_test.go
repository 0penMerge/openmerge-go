package openmerge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyWebhookSignature(t *testing.T) {
	payload := []byte(`{"event":"record.updated"}`)
	timestamp := int64(1_750_000_000)
	secret := "whsec_test"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	verified, err := VerifyWebhookSignature(payload, fmt.Sprintf("t=%d,v1=%064d,v1=%s", timestamp, 0, signature), secret, 5*time.Minute, time.Unix(timestamp, 0))
	if err != nil || string(verified.Payload) != string(payload) {
		t.Fatalf("verification failed: %v", err)
	}
	if _, err := VerifyWebhookSignature(append(payload, ' '), fmt.Sprintf("t=%d,v1=%s", timestamp, signature), secret, 5*time.Minute, time.Unix(timestamp, 0)); err == nil {
		t.Fatal("modified payload was accepted")
	}
}
