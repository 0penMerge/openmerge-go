package openmerge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatalogAndAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer om_test_secret" {
			t.Fatal("missing authorization header")
		}
		if request.URL.Query().Get("wsid") != "ws_1" {
			t.Fatal("missing workspace query")
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"integrations": []ConnectorIntegration{{
			Provider: "hubspot", Descriptor: ConnectorDescriptor{ID: "hubspot", Name: "HubSpot"},
		}}})
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	integrations, err := client.ListIntegrations(context.Background(), "ws_1")
	if err != nil || integrations[0].Provider != "hubspot" {
		t.Fatalf("unexpected result: %#v %v", integrations, err)
	}
}

func TestIdempotentWriteRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if request.Header.Get("Idempotency-Key") != "customer-event-123" {
			t.Fatal("missing idempotency key")
		}
		if attempts == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"write":{"id":"w_1","workflow_id":"wf_1","state":"queued"}}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL, RetryBaseDelay: time.Nanosecond})
	write, err := client.SubmitWriteback(context.Background(), "Contact", "u_1", "ws_1", "la_1", map[string]any{"email": "new@example.com"}, "customer-event-123")
	if err != nil || write.ID != "w_1" || attempts != 2 {
		t.Fatalf("unexpected write: %#v attempts=%d error=%v", write, attempts, err)
	}
}

func TestDeveloperIRAndConnectionMappingContracts(t *testing.T) {
	requests := make([]*http.Request, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Clone(request.Context()))
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/developer-ir/oa_1/Contact":
			if request.URL.Query().Get("wsid") != "ws_1" {
				t.Fatal("missing workspace query")
			}
			_, _ = writer.Write([]byte(`{"wsid":"ws_1","oauth_app_id":"oa_1","model_id":"Contact","generation":1,"document_hash":"sha256:test","document":{},"requirements":{},"removed_fields":[]}`))
		case request.URL.Path == "/linked-accounts/la_1/field-mapping-token":
			_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://connect.openmerge.dev/x"}`))
		default:
			_, _ = writer.Write([]byte(`{"id":"map_1","action":"activate","state":"queued"}`))
		}
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	developer, err := client.UpdateDeveloperIR(context.Background(), "ws_1", "oa_1", "Contact", 0, map[string]map[string]any{
		"customer_tier": {"type": "string", "required": true},
	}, nil)
	if err != nil || developer.Generation != 1 {
		t.Fatalf("unexpected developer IR: %#v error=%v", developer, err)
	}
	token, err := client.CreateConnectionMappingToken(context.Background(), "la_1", "https://customer.example")
	if err != nil || token.ExpiresIn != 1800 {
		t.Fatalf("unexpected mapping token: %#v error=%v", token, err)
	}
	job, err := client.ActivateConnectionMapping(context.Background(), "la_1", map[string]int{"Contact": 1}, map[string]map[string]string{
		"Contact": {"customer_tier": "Customer_Tier__c"},
	}, "mapping-1")
	if err != nil || job.State != "queued" || len(requests) != 3 {
		t.Fatalf("unexpected mapping job: %#v requests=%d error=%v", job, len(requests), err)
	}
}
