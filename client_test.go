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
		_ = json.NewEncoder(writer).Encode([]ConnectorIntegration{{
			Provider: "hubspot", Descriptor: ConnectorDescriptor{ID: "hubspot", Name: "HubSpot"},
		}})
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
