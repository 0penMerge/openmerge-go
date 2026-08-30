package openmerge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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

func TestVerifiedWebhookEnvelopeContract(t *testing.T) {
	payload := []byte(`{"event":"record.updated","ctx":{"workspace":{"id":"ws_1","name":"Production"},"user":{"origin_id":"customer_42","actor_id":null},"integration":{"provider":"salesforce","name":"Salesforce","categories":["crm"]},"oauth_app":{"id":"oa_1","name":"Primary"},"connection":{"id":"la_1"}},"data":{"model":"Contact","remote_id":"003","unified_id":"contact_1","record":{"email":"person@example.test"},"deleted_at":null},"breadcrumbs":{"event_id":"evt_1","occurred_at":"2026-08-31T00:00:00Z","content_hash":"sha256:value","sync_run_id":"run_1","source_cursor":{"page":2},"source_updated_at":"2026-08-30T23:59:00Z","plan":{"generation":7,"hash":"plan_7"}}}`)
	timestamp := int64(1_750_000_000)
	secret := "whsec_test"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(payload)
	verified, err := VerifyWebhookSignature(
		payload,
		fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil))),
		secret,
		5*time.Minute,
		time.Unix(timestamp, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	var envelope RecordWebhookEnvelope
	if err := json.Unmarshal(verified.Payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Event != RecordWebhookEventUpdated || envelope.Ctx.Workspace.ID != "ws_1" {
		t.Fatalf("unexpected webhook context: %#v", envelope)
	}
	if envelope.Ctx.User.OriginID == nil || *envelope.Ctx.User.OriginID != "customer_42" {
		t.Fatalf("unexpected webhook user: %#v", envelope.Ctx.User)
	}
	if envelope.Data.Model != "Contact" || envelope.Data.Record["email"] != "person@example.test" {
		t.Fatalf("unexpected webhook data: %#v", envelope.Data)
	}
	if envelope.Breadcrumbs.ContentHash != "sha256:value" || envelope.Breadcrumbs.Plan.Generation == nil || *envelope.Breadcrumbs.Plan.Generation != 7 {
		t.Fatalf("unexpected webhook breadcrumbs: %#v", envelope.Breadcrumbs)
	}
}

func TestDomainWebhookEnvelopeContract(t *testing.T) {
	payload := []byte(`{"event":"sync.completed","ctx":{"workspace":{"id":"ws_1"},"user":{"origin_id":"customer_42","actor_id":"user_1"},"integration":{"provider":"hubspot","name":"HubSpot","categories":["crm"]},"oauth_app":{"id":"oa_1"},"connection":{"id":"la_1"}},"data":{"linked_account_id":"la_1","records_written":42,"checkpoint":{"cursor":"next-page"}},"breadcrumbs":{"event_id":"evt_domain_1","occurred_at":"2026-08-31T00:00:00Z","entity":{"type":"sync_run","id":"run_1"},"request_id":"request_1"}}`)
	var envelope DomainWebhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Event != DomainWebhookEventSyncCompleted || envelope.Data["records_written"] != float64(42) {
		t.Fatalf("unexpected domain webhook: %#v", envelope)
	}
	if envelope.Breadcrumbs.Entity.Type == nil || *envelope.Breadcrumbs.Entity.Type != "sync_run" || envelope.Breadcrumbs.RequestID == nil || *envelope.Breadcrumbs.RequestID != "request_1" {
		t.Fatalf("unexpected domain breadcrumbs: %#v", envelope.Breadcrumbs)
	}
	if envelope.Ctx.Connection.ID == nil || *envelope.Ctx.Connection.ID != "la_1" {
		t.Fatalf("unexpected domain context: %#v", envelope.Ctx)
	}
}

func TestTypedWebhookEnvelopesRetainRequiredNullableKeys(t *testing.T) {
	recordRaw, err := json.Marshal(RecordWebhookEnvelope{
		Data: RecordWebhookData{Record: map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(recordRaw, &record); err != nil {
		t.Fatal(err)
	}
	recordContext := record["ctx"].(map[string]any)
	recordWorkspace := recordContext["workspace"].(map[string]any)
	recordUser := recordContext["user"].(map[string]any)
	recordIntegration := recordContext["integration"].(map[string]any)
	recordOAuth := recordContext["oauth_app"].(map[string]any)
	recordData := record["data"].(map[string]any)
	recordBreadcrumbs := record["breadcrumbs"].(map[string]any)
	recordPlan := recordBreadcrumbs["plan"].(map[string]any)
	for key, object := range map[string]map[string]any{
		"workspace.name":                recordWorkspace,
		"user.origin_id":                recordUser,
		"user.actor_id":                 recordUser,
		"integration.provider":          recordIntegration,
		"integration.name":              recordIntegration,
		"integration.categories":        recordIntegration,
		"oauth_app.id":                  recordOAuth,
		"oauth_app.name":                recordOAuth,
		"data.deleted_at":               recordData,
		"breadcrumbs.sync_run_id":       recordBreadcrumbs,
		"breadcrumbs.source_cursor":     recordBreadcrumbs,
		"breadcrumbs.source_updated_at": recordBreadcrumbs,
		"plan.generation":               recordPlan,
		"plan.hash":                     recordPlan,
	} {
		field := key[strings.LastIndex(key, ".")+1:]
		if _, exists := object[field]; !exists {
			t.Errorf("required record webhook key %s was omitted: %s", key, recordRaw)
		}
	}

	domainRaw, err := json.Marshal(DomainWebhookEnvelope{Data: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	var domain map[string]any
	if err := json.Unmarshal(domainRaw, &domain); err != nil {
		t.Fatal(err)
	}
	domainContext := domain["ctx"].(map[string]any)
	domainUser := domainContext["user"].(map[string]any)
	domainIntegration := domainContext["integration"].(map[string]any)
	domainOAuth := domainContext["oauth_app"].(map[string]any)
	domainConnection := domainContext["connection"].(map[string]any)
	domainBreadcrumbs := domain["breadcrumbs"].(map[string]any)
	domainEntity := domainBreadcrumbs["entity"].(map[string]any)
	for key, object := range map[string]map[string]any{
		"user.origin_id":          domainUser,
		"user.actor_id":           domainUser,
		"integration.provider":    domainIntegration,
		"integration.name":        domainIntegration,
		"integration.categories":  domainIntegration,
		"oauth_app.id":            domainOAuth,
		"connection.id":           domainConnection,
		"entity.type":             domainEntity,
		"entity.id":               domainEntity,
		"breadcrumbs.occurred_at": domainBreadcrumbs,
		"breadcrumbs.request_id":  domainBreadcrumbs,
	} {
		field := key[strings.LastIndex(key, ".")+1:]
		if _, exists := object[field]; !exists {
			t.Errorf("required domain webhook key %s was omitted: %s", key, domainRaw)
		}
	}
}
