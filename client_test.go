package openmerge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestUnifiedRecordActionContracts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || !strings.HasPrefix(request.URL.Path, "/unified/Contact/actions/") {
			t.Errorf("unexpected action request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Idempotency-Key") == "" {
			t.Error("missing idempotency key")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body["wsid"] != "ws_1" || body["linked_account_id"] != "la_1" {
			t.Errorf("missing action scope: %#v", body)
		}
		switch request.URL.Path {
		case "/unified/Contact/actions/create":
			if body["changes"].(map[string]any)["email"] != "person@example.test" {
				t.Errorf("unexpected create body: %#v", body)
			}
		case "/unified/Contact/actions/upsert":
			if body["unified_id"] != "contact_1" {
				t.Errorf("unexpected upsert body: %#v", body)
			}
		case "/unified/Contact/actions/delete":
			if body["unified_id"] != "contact_1" {
				t.Errorf("unexpected delete body: %#v", body)
			}
		case "/unified/Contact/actions/bulk":
			items := body["items"].([]any)
			if len(items) != 2 || items[1].(map[string]any)["changes"] == nil {
				t.Errorf("unexpected bulk body: %#v", body)
			}
		case "/unified/Contact/actions/custom-fields":
			if body["definition"].(map[string]any)["name"] != "Research_Bio__c" {
				t.Errorf("unexpected custom-field body: %#v", body)
			}
		default:
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"write":{"id":"write_1","workflow_id":"workflow_1","state":"queued","created_at":10.5,"updated_at":11.5}}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	scope := RecordActionScope{WorkspaceID: "ws_1", LinkedAccountID: "la_1", Model: "Contact"}

	scope.IdempotencyKey = "create-001"
	created, err := client.CreateRecord(context.Background(), CreateRecordParams{
		RecordActionScope: scope,
		Changes:           map[string]any{"email": "person@example.test"},
	})
	if err != nil || created.CreatedAt == nil || *created.CreatedAt != 10.5 || created.UpdatedAt == nil || *created.UpdatedAt != 11.5 {
		t.Fatalf("unexpected create action: %#v error=%v", created, err)
	}
	scope.IdempotencyKey = "upsert-001"
	if _, err := client.UpsertRecord(context.Background(), UpsertRecordParams{
		RecordActionScope: scope, UnifiedID: "contact_1", Changes: map[string]any{"email": "new@example.test"},
	}); err != nil {
		t.Fatal(err)
	}
	scope.IdempotencyKey = "delete-001"
	if _, err := client.DeleteRecord(context.Background(), DeleteRecordParams{
		RecordActionScope: scope, UnifiedID: "contact_1",
	}); err != nil {
		t.Fatal(err)
	}
	scope.IdempotencyKey = "bulk-0001"
	if _, err := client.BulkRecords(context.Background(), BulkRecordActionParams{
		RecordActionScope: scope,
		Items: []BulkRecordActionItem{
			{Operation: BulkRecordDelete, UnifiedID: "contact_2"},
			{Operation: BulkRecordDelete, RemoteID: "003xx0000000002"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	scope.IdempotencyKey = "custom-001"
	if _, err := client.CreateCustomField(context.Background(), CreateCustomFieldParams{
		RecordActionScope: scope,
		Definition:        map[string]any{"name": "Research_Bio__c", "type": "string"},
	}); err != nil {
		t.Fatal(err)
	}
	if requests != 5 {
		t.Fatalf("expected five record action requests, got %d", requests)
	}
	if _, err := client.BulkRecords(context.Background(), BulkRecordActionParams{RecordActionScope: scope}); err == nil {
		t.Fatal("accepted empty bulk action")
	}
	if requests != 5 {
		t.Fatal("invalid bulk action reached the network")
	}
	invalid := []struct {
		name string
		call func() error
	}{
		{"blank scope", func() error {
			_, err := client.CreateRecord(context.Background(), CreateRecordParams{
				RecordActionScope: RecordActionScope{WorkspaceID: " ", LinkedAccountID: "la_1", Model: "Contact", IdempotencyKey: "invalid-001"},
				Changes:           map[string]any{"email": "person@example.test"},
			})
			return err
		}},
		{"invalid model", func() error {
			bad := scope
			bad.Model = "Contact.v2"
			_, err := client.CreateRecord(context.Background(), CreateRecordParams{RecordActionScope: bad, Changes: map[string]any{"email": "person@example.test"}})
			return err
		}},
		{"blank idempotency key", func() error {
			bad := scope
			bad.IdempotencyKey = "        "
			_, err := client.CreateRecord(context.Background(), CreateRecordParams{RecordActionScope: bad, Changes: map[string]any{"email": "person@example.test"}})
			return err
		}},
		{"empty create", func() error {
			_, err := client.CreateRecord(context.Background(), CreateRecordParams{RecordActionScope: scope})
			return err
		}},
		{"blank optional upsert identity", func() error {
			_, err := client.UpsertRecord(context.Background(), UpsertRecordParams{
				RecordActionScope: scope, UnifiedID: " ", Changes: map[string]any{"email": "person@example.test"},
			})
			return err
		}},
		{"empty upsert", func() error {
			_, err := client.UpsertRecord(context.Background(), UpsertRecordParams{RecordActionScope: scope})
			return err
		}},
		{"blank delete identity", func() error {
			_, err := client.DeleteRecord(context.Background(), DeleteRecordParams{RecordActionScope: scope, UnifiedID: " "})
			return err
		}},
		{"mixed bulk", func() error {
			_, err := client.BulkRecords(context.Background(), BulkRecordActionParams{RecordActionScope: scope, Items: []BulkRecordActionItem{
				{Operation: BulkRecordCreate, Changes: map[string]any{"email": "person@example.test"}},
				{Operation: BulkRecordDelete, UnifiedID: "contact_1"},
			}})
			return err
		}},
		{"update without identity", func() error {
			_, err := client.BulkRecords(context.Background(), BulkRecordActionParams{RecordActionScope: scope, Items: []BulkRecordActionItem{
				{Operation: BulkRecordUpdate, Changes: map[string]any{"email": "person@example.test"}},
			}})
			return err
		}},
		{"delete with changes", func() error {
			_, err := client.BulkRecords(context.Background(), BulkRecordActionParams{RecordActionScope: scope, Items: []BulkRecordActionItem{
				{Operation: BulkRecordDelete, UnifiedID: "contact_1", Changes: map[string]any{"ignored": true}},
			}})
			return err
		}},
		{"ambiguous identity", func() error {
			_, err := client.BulkRecords(context.Background(), BulkRecordActionParams{RecordActionScope: scope, Items: []BulkRecordActionItem{
				{Operation: BulkRecordUpsert, UnifiedID: "contact_1", RemoteID: "003", Changes: map[string]any{"email": "person@example.test"}},
			}})
			return err
		}},
		{"empty custom field", func() error {
			_, err := client.CreateCustomField(context.Background(), CreateCustomFieldParams{RecordActionScope: scope})
			return err
		}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("invalid record action was accepted")
			}
		})
	}
	if requests != 5 {
		t.Fatal("an invalid record action reached the network")
	}
}

func TestLinkTokenCarriesApplicationMappingOverrides(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://connect.openmerge.dev/x"}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	_, err := client.CreateLinkTokenWithOptions(
		context.Background(),
		"ws_1",
		"customer_1",
		LinkTokenOptions{MappingOverrides: map[string]map[string]map[string]string{
			"salesforce": {"Contact": {"research_bio": "Research_Bio__c"}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	models, ok := body["mapping_overrides"].(map[string]any)
	if !ok {
		t.Fatalf("mapping_overrides missing from request: %#v", body)
	}
	salesforce, ok := models["salesforce"].(map[string]any)
	if !ok {
		t.Fatalf("salesforce mapping overrides missing: %#v", models)
	}
	contact, ok := salesforce["Contact"].(map[string]any)
	if !ok || contact["research_bio"] != "Research_Bio__c" {
		t.Fatalf("unexpected mapping override: %#v", models)
	}
	if _, exists := body["host_origin"]; exists {
		t.Fatalf("empty host origin must be omitted for top-level hosted links: %#v", body)
	}
	if _, exists := body["appearance"]; exists {
		t.Fatalf("unset appearance must be omitted: %#v", body)
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
			_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://widgets.openmerge.dev/connect/tttttttttttttttttttttttttttttttt?mode=field-mapping","discovery_job_id":"map_discovery_1"}`))
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
	if err != nil || token.ExpiresIn != 1800 || token.DiscoveryJobID != "map_discovery_1" {
		t.Fatalf("unexpected mapping token: %#v error=%v", token, err)
	}
	job, err := client.ActivateConnectionMapping(context.Background(), "la_1", map[string]int{"Contact": 1}, map[string]map[string]string{
		"Contact": {"customer_tier": "Customer_Tier__c"},
	}, "mapping-1")
	if err != nil || job.State != "queued" || len(requests) != 3 {
		t.Fatalf("unexpected mapping job: %#v requests=%d error=%v", job, len(requests), err)
	}
}

func TestTypedDeveloperIRFieldContract(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		_, _ = writer.Write([]byte(`{"wsid":"ws_1","oauth_app_id":"oa_1","model_id":"Contact","generation":2,"document_hash":"sha256:test","document":{},"requirements":{},"removed_fields":[]}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	nullable := false
	result, err := client.UpdateDeveloperIRWithFields(context.Background(), "ws_1", "oa_1", "Contact", 1, map[string]DeveloperIRField{
		"research_bio": {
			Type:                 DeveloperFieldString,
			Label:                "Research bio",
			Required:             true,
			Direction:            DeveloperFieldTwoWay,
			Format:               "markdown",
			Description:          "Application research context",
			Nullable:             &nullable,
			DefaultProviderField: "Research_Bio__c",
			OverrideBy:           []DeveloperFieldOverride{DeveloperFieldOverrideApplication, DeveloperFieldOverrideCustomer},
			Items:                map[string]any{"x-openmerge-hint": "preserved"},
			Values:               []string{"short", "long"},
			Ref:                  "#/models/ResearchBio",
			Sensitivity:          DeveloperFieldSensitivityPII,
		},
	}, nil)
	if err != nil || result.Generation != 2 {
		t.Fatalf("unexpected typed developer IR: %#v error=%v", result, err)
	}
	field := body["fields"].(map[string]any)["research_bio"].(map[string]any)
	if field["direction"] != "two_way" || field["nullable"] != false || field["sensitivity"] != "pii" {
		t.Fatalf("typed field contract missing values: %#v", field)
	}
	if field["items"].(map[string]any)["x-openmerge-hint"] != "preserved" {
		t.Fatalf("nested field metadata was not preserved: %#v", field)
	}
}

func TestDeveloperIRRevisionAndMappingMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/developer-ir/oa_1/Contact":
			_, _ = writer.Write([]byte(`{"wsid":"ws_1","oauth_app_id":"oa_1","provider":"salesforce","model_id":"Contact","generation":2,"base_hash":"base","document_hash":"current","inherited":false,"document":{"fields":{}},"base_document":{"fields":{"email":{"type":"string"}}},"requirements":{},"removed_fields":[],"actor_id":"user_1","created_at":99,"updated_at":100}`))
		case "/developer-ir/oa_1/Contact/revisions":
			if request.URL.Query().Get("wsid") != "ws_1" || request.URL.Query().Get("limit") != "10" {
				t.Errorf("unexpected revision query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"revisions":[{"head_id":"ws_1:oa_1:Contact","wsid":"ws_1","provider":"salesforce","oauth_app_id":"oa_1","model_id":"Contact","generation":2,"base_hash":"base","document_hash":"current","contract_hash":"contract","document":{"fields":{}},"requirements":{},"removed_fields":[],"actor_id":"user_1","created_at":100.25}]}`))
		case "/linked-accounts/la_1/field-mapping":
			_, _ = writer.Write([]byte(`{"linked_account":{"id":"la_1"},"oauth_app":{"id":"oa_1"},"status":"active","required":true,"models":[{"id":"Contact","schema_ready":true,"fields":[{"id":"research_bio","label":"Research bio","type":"string","required":false,"requirement_source":"developer_ir","provider_create_required":true,"direction":"two_way"}],"provider_fields":[{"path":"Research_Bio__c","type":"string","vendor_extension":{"precision":100}}],"provider_create_requirements":[{"path":"Research_Bio__c","label":"Research bio","type":"string","mapped_by":["research_bio"],"satisfied":true}],"write_readiness":{"create":true,"upsert":true,"missing_create_fields":[]}}],"generated_at":"2026-08-31T00:00:00Z"}`))
		case "/connection-mapping-jobs/map_1":
			_, _ = writer.Write([]byte(`{"id":"map_1","action":"discover","state":"failed","linked_account_id":"la_1","models":{"Contact":{"cursor":"page_2"}},"result":{"partial":true},"error_type":"ProviderRateLimit","error":"retry later","created_at":10,"updated_at":20,"finished_at":21}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})

	ir, err := client.GetDeveloperIR(context.Background(), "ws_1", "oa_1", "Contact")
	if err != nil || ir.BaseDocument["fields"] == nil || ir.BaseHash != "base" || ir.UpdatedAt == nil || *ir.UpdatedAt != 100 {
		t.Fatalf("base document was not retained: %#v error=%v", ir, err)
	}
	revisions, err := client.ListDeveloperIRRevisions(context.Background(), "ws_1", "oa_1", "Contact", 10)
	if err != nil || len(revisions) != 1 || revisions[0].ContractHash != "contract" || revisions[0].CreatedAt != 100.25 {
		t.Fatalf("unexpected revisions: %#v error=%v", revisions, err)
	}
	if _, err := client.ListDeveloperIRRevisions(context.Background(), "ws_1", "oa_1", "Contact", 101); err == nil {
		t.Fatal("accepted out-of-range revision limit")
	}
	mapping, err := client.GetConnectionMapping(context.Background(), "la_1")
	if err != nil || mapping.GeneratedAt == "" || mapping.Models[0].ProviderFields[0]["vendor_extension"] == nil || !mapping.Models[0].WriteReadiness.Create || !mapping.Models[0].Fields[0].ProviderCreateRequired {
		t.Fatalf("mapping schema metadata was lost: %#v error=%v", mapping, err)
	}
	job, err := client.GetConnectionMappingJob(context.Background(), "map_1")
	if err != nil || job.ErrorType != "ProviderRateLimit" || job.CreatedAt == nil || job.UpdatedAt == nil || job.FinishedAt == nil || job.Models == nil {
		t.Fatalf("mapping job metadata was lost: %#v error=%v", job, err)
	}
}

func TestHostedWidgetTokenContracts(t *testing.T) {
	radius := 0.0
	scale := 1.1
	appearance := &WidgetAppearance{
		Mode:    WidgetColorModeDark,
		Density: WidgetDensityCompact,
		Colors: &WidgetColors{
			Primary:      "#65a30d",
			SurfaceHover: "#18181b",
			BorderStrong: "#52525b",
		},
		Typography: &WidgetTypography{FontFamily: "Inter, sans-serif", Scale: &scale},
		Shape:      &WidgetShape{Radius: &radius},
		Branding:   &WidgetBranding{ProductName: "Acme CRM", LogoURL: "https://cdn.example/logo.svg"},
	}
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-OpenMerge-SDK") != "go/0.3.0" {
			t.Errorf("unexpected SDK header: %q", request.Header.Get("X-OpenMerge-SDK"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		requests = append(requests, body)
		writer.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/field-mapping-token") {
			_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://widgets.openmerge.dev/connect/tttttttttttttttttttttttttttttttt?mode=field-mapping","discovery_job_id":"map_1"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://widgets.openmerge.dev/connect/tttttttttttttttttttttttttttttttt"}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})

	link, err := client.CreateLinkTokenWithOptions(context.Background(), "ws_1", "customer_1", LinkTokenOptions{
		AllowedCategories: []string{"crm"},
		HostOrigin:        "https://customer.example",
		Appearance:        appearance,
	})
	if err != nil || link.HostedURL == "" {
		t.Fatalf("unexpected link token: %#v error=%v", link, err)
	}
	reconnect, err := client.CreateReconnectToken(context.Background(), "ws_1", "la_1", ReconnectTokenOptions{
		EndUserOriginID: "customer_1",
		HostOrigin:      "chrome-extension://abcdefghijklmnop",
		Appearance:      appearance,
		MappingOverrides: map[string]map[string]map[string]string{
			"salesforce": {"Contact": {"research_bio": "Research_Bio__c"}},
		},
	})
	if err != nil || reconnect.Token == "" {
		t.Fatalf("unexpected reconnect token: %#v error=%v", reconnect, err)
	}
	mapping, err := client.CreateConnectionMappingTokenWithOptions(context.Background(), "la_1", ConnectionMappingTokenOptions{
		HostOrigin: "https://customer.example",
		Appearance: appearance,
	})
	if err != nil || mapping.DiscoveryJobID != "map_1" {
		t.Fatalf("unexpected mapping token: %#v error=%v", mapping, err)
	}
	if _, err := client.CreateConnectionMappingToken(context.Background(), " ", ""); err == nil {
		t.Fatal("accepted a blank mapping linked-account ID")
	}
	if len(requests) != 3 {
		t.Fatalf("expected three token requests, got %d", len(requests))
	}
	colors := requests[0]["appearance"].(map[string]any)["colors"].(map[string]any)
	if colors["surfaceHover"] != "#18181b" || colors["borderStrong"] != "#52525b" {
		t.Fatalf("canonical appearance fields missing: %#v", colors)
	}
	if requests[0]["appearance"].(map[string]any)["shape"].(map[string]any)["radius"] != float64(0) {
		t.Fatalf("zero radius was dropped: %#v", requests[0])
	}
	if requests[1]["linked_account_id"] != "la_1" || requests[1]["end_user_origin_id"] != "customer_1" {
		t.Fatalf("reconnect affinity fields missing: %#v", requests[1])
	}
	if requests[2]["host_origin"] != "https://customer.example" || requests[2]["appearance"] == nil {
		t.Fatalf("mapping token options missing: %#v", requests[2])
	}

	fieldMappingURL, err := HostedWidgetURL(link.HostedURL, HostedWidgetFieldMapping, "CustomContact_2")
	if err != nil || fieldMappingURL != "https://widgets.openmerge.dev/connect/tttttttttttttttttttttttttttttttt?mode=field-mapping&model=CustomContact_2" {
		t.Fatalf("unexpected hosted widget URL: %q error=%v", fieldMappingURL, err)
	}
	if _, err := HostedWidgetURL("http://widgets.example/connect/tttttttttttttttt", HostedWidgetConnect, ""); err == nil {
		t.Fatal("accepted cleartext non-loopback widget URL")
	}
	if _, err := HostedWidgetURL(link.HostedURL, HostedWidgetMode("unknown"), ""); err == nil {
		t.Fatal("accepted unknown widget mode")
	}
	preservedModeURL, err := HostedWidgetURL(mapping.HostedURL, "", "")
	if err != nil {
		t.Fatalf("server-issued mapping mode was rejected: %v", err)
	}
	preserved, _ := url.Parse(preservedModeURL)
	if preserved.Query().Get("mode") != "field-mapping" {
		t.Fatalf("server-issued mapping mode was not preserved: %s", preservedModeURL)
	}
}

func TestLinkedAccountLifecycleContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "GET /linked-accounts/la_1":
			_, _ = writer.Write([]byte(`{"id":"la_1","wsid":"ws_1","provider":"hubspot","end_user_origin_id":"customer_1","status":"healthy","created_at":1}`))
		case "POST /linked-accounts/la_1/pause":
			_, _ = writer.Write([]byte(`{"id":"la_1","status":"paused","cancelled_runs":2}`))
		case "POST /linked-accounts/la_1/resume":
			_, _ = writer.Write([]byte(`{"id":"la_1","status":"healthy","runs":[{"id":"run_1","workflow_id":"wf_1","state":"queued"}]}`))
		case "PUT /linked-accounts/la_1/schedule":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["cadence_seconds"] != float64(3600) || body["reconciliation_seconds_by_model"].(map[string]any)["Contact"] != float64(86400) {
				t.Errorf("unexpected schedule body: %#v", body)
			}
			_, _ = writer.Write([]byte(`{"id":"la_1","cadence_seconds":3600,"scheduled_models":["Contact"],"reconciliation_seconds_by_model":{"Contact":86400}}`))
		case "DELETE /linked-accounts/la_1":
			_, _ = writer.Write([]byte(`{"deleted":true,"cancelled_runs":1,"erasure_job_id":"erase:ws_1:la_1","erasure_state":"pending"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	account, err := client.GetLinkedAccount(context.Background(), "la_1")
	if err != nil || account.Provider != "hubspot" {
		t.Fatalf("unexpected linked account: %#v error=%v", account, err)
	}
	paused, err := client.PauseLinkedAccount(context.Background(), "la_1")
	if err != nil || paused.Status != "paused" || paused.CancelledRuns == nil || *paused.CancelledRuns != 2 {
		t.Fatalf("unexpected pause response: %#v error=%v", paused, err)
	}
	resumed, err := client.ResumeLinkedAccount(context.Background(), "la_1")
	if err != nil || len(resumed.Runs) != 1 || resumed.Runs[0].WorkflowID != "wf_1" {
		t.Fatalf("unexpected resume response: %#v error=%v", resumed, err)
	}
	schedule, err := client.UpdateLinkedAccountSchedule(context.Background(), "la_1", 3600, UpdateScheduleOptions{
		ReconciliationSecondsByModel: map[string]int{"Contact": 86400},
	})
	if err != nil || schedule.ReconciliationSecondsByModel["Contact"] != 86400 {
		t.Fatalf("unexpected schedule: %#v error=%v", schedule, err)
	}
	deleted, err := client.DeleteLinkedAccount(context.Background(), "la_1")
	if err != nil || !deleted.Deleted || deleted.ErasureJobID == "" {
		t.Fatalf("unexpected delete response: %#v error=%v", deleted, err)
	}
}

func TestRecordOptionsIterationAndBackwardCompatibility(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		query := request.URL.Query()
		if query.Get("wsid") != "ws_1" || query.Get("linked_account_id") != "la_1" {
			t.Errorf("record scope missing: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch query.Get("cursor") {
		case "legacy":
			if query.Get("page_size") != "25" {
				t.Errorf("legacy page size missing: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"records":[],"next_cursor":null}`))
		case "direct":
			if query.Get("include_remote_data") != "true" || query.Get("include_deleted") != "true" {
				t.Errorf("record include flags missing: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"records":[{"wsid":"ws_1","linked_account_id":"la_1","model":"Contact","unified_id":"u_direct","data":{},"remote_data":{"id":"remote"},"deleted_at":170,"deletion_source":"hubspot_archived","updated_at":172.5}],"next_cursor":null}`))
		case "page_2":
			_, _ = writer.Write([]byte(`{"records":[{"wsid":"ws_1","linked_account_id":"la_1","model":"Contact","unified_id":"u_2","data":{}}],"next_cursor":null}`))
		default:
			_, _ = writer.Write([]byte(`{"records":[{"wsid":"ws_1","linked_account_id":"la_1","model":"Contact","unified_id":"u_1","data":{}}],"next_cursor":"page_2"}`))
		}
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	if _, err := client.ListRecords(context.Background(), "Contact", "ws_1", "la_1", "legacy", 25); err != nil {
		t.Fatalf("backward-compatible ListRecords failed: %v", err)
	}
	direct, err := client.ListRecordsWithOptions(context.Background(), "Contact", "ws_1", "la_1", ListRecordsOptions{
		Cursor: "direct", PageSize: 50, IncludeRemoteData: true, IncludeDeleted: true,
	})
	if err != nil || len(direct.Records) != 1 || direct.Records[0].RemoteData == nil || direct.Records[0].Provider != "" || direct.Records[0].DeletionSource != "hubspot_archived" || direct.Records[0].UpdatedAt == nil || *direct.Records[0].UpdatedAt != 172.5 {
		t.Fatalf("unexpected direct page: %#v error=%v", direct, err)
	}
	encoded, err := json.Marshal(direct.Records[0])
	if err != nil || strings.Contains(string(encoded), `"provider"`) {
		t.Fatalf("provider must stay optional on the V1 wire contract: %s error=%v", encoded, err)
	}
	var ids []string
	err = client.IterateRecords(context.Background(), "Contact", "ws_1", "la_1", ListRecordsOptions{PageSize: 100}, func(record UnifiedRecord) error {
		ids = append(ids, record.UnifiedID)
		return nil
	})
	if err != nil || strings.Join(ids, ",") != "u_1,u_2" || requests != 4 {
		t.Fatalf("unexpected iteration: ids=%v requests=%d error=%v", ids, requests, err)
	}
}

func TestWaitForWriteback(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		state := "queued"
		if attempts >= 2 {
			state = "completed"
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"write": map[string]any{"id": "w_1", "workflow_id": "wf_1", "state": state},
		})
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	write, err := client.WaitForWriteback(context.Background(), "w_1", "ws_1", WaitForWritebackOptions{
		Interval: 10 * time.Millisecond,
		Timeout:  time.Second,
	})
	if err != nil || write.State != "completed" || attempts != 2 {
		t.Fatalf("unexpected terminal writeback: %#v attempts=%d error=%v", write, attempts, err)
	}
	_, err = client.WaitForWriteback(context.Background(), "w_1", "ws_1", WaitForWritebackOptions{
		Interval:       10 * time.Millisecond,
		Timeout:        time.Second,
		TerminalStates: []string{},
	})
	if err == nil {
		t.Fatal("accepted an explicitly empty terminal-state set")
	}

	blocked := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"write": map[string]any{"id": "w_2", "workflow_id": "wf_2", "state": "queued"},
		})
	}))
	defer blocked.Close()
	blockedClient, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: blocked.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = blockedClient.WaitForWriteback(ctx, "w_2", "ws_1", WaitForWritebackOptions{
		Interval: 10 * time.Millisecond,
		Timeout:  time.Second,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestHostedWidgetURLRejectsMalformedValues(t *testing.T) {
	values := []string{
		"https://user:pass@widgets.example/connect/tttttttttttttttt",
		"https://widgets.example/not-connect/tttttttttttttttt",
		"https://widgets.example/prefix/connect/tttttttttttttttt",
		"https://widgets.example/connect/short",
		"https://widgets.example/connect/tttttttttttttttt#fragment",
	}
	for _, value := range values {
		if _, err := HostedWidgetURL(value, HostedWidgetManager, ""); err == nil {
			t.Errorf("accepted malformed hosted URL: %s", value)
		}
	}
	loopback, err := HostedWidgetURL("http://localhost:3001/connect/tttttttttttttttt", HostedWidgetSyncStatus, "")
	if err != nil {
		t.Fatalf("loopback hosted URL rejected: %v", err)
	}
	parsed, _ := url.Parse(loopback)
	if parsed.Query().Get("mode") != "sync-status" {
		t.Fatalf("unexpected loopback URL: %s", loopback)
	}
	if _, err := HostedWidgetURL("https://widgets.openmerge.dev/connect/tttttttttttttttt", HostedWidgetFieldMapping, "Contact.v2"); err == nil {
		t.Fatal("accepted a non-canonical model identifier")
	}
	selfHosted := "https://widgets.customer.example/connect/tttttttttttttttt?mode=manager"
	if _, err := HostedWidgetURL(selfHosted, "", ""); err == nil {
		t.Fatal("accepted an unconfigured self-hosted widget origin")
	}
	allowed, err := HostedWidgetURL(selfHosted, "", "", "https://widgets.customer.example")
	if err != nil {
		t.Fatalf("explicit self-hosted widget origin was rejected: %v", err)
	}
	allowedURL, _ := url.Parse(allowed)
	if allowedURL.Query().Get("mode") != "manager" {
		t.Fatalf("self-hosted server mode was not preserved: %s", allowed)
	}
	if _, err := HostedWidgetURL(selfHosted, "", "", "https://widgets.customer.example/tenant"); err == nil {
		t.Fatal("accepted an allowed origin containing a path")
	}
	if _, err := HostedWidgetURL("https://widgets.openmerge.dev/connect/tttttttttttttttt?mode=unknown", "", ""); err == nil {
		t.Fatal("accepted an invalid server-issued widget mode")
	}
}

func TestMappingTokenRequiresDiscoveryJob(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		_, _ = writer.Write([]byte(`{"token":"tttttttttttttttttttttttttttttttt","expires_in":1800,"hosted_url":"https://widgets.openmerge.dev/connect/tttttttttttttttttttttttttttttttt?mode=field-mapping"}`))
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "om_test_secret", BaseURL: server.URL})
	if _, err := client.CreateConnectionMappingToken(context.Background(), "la_1", ""); err == nil {
		t.Fatal("accepted a mapping token response without a discovery job ID")
	}
	if requests != 1 {
		t.Fatalf("unexpected request count: %d", requests)
	}
}

func TestMailContentIsBinaryAndOwnerScoped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("end_user_origin_id") != "customer" {
			t.Error("missing customer scope")
		}
		_, _ = w.Write([]byte{0, 255})
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "om_test", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	data, err := client.GetMailContent(context.Background(), MailScope{WorkspaceID: "ws", OwnerID: "customer", AccountID: "account"}, "message", "raw")
	if err != nil || len(data) != 2 || data[1] != 255 {
		t.Fatalf("binary content: %v %v", data, err)
	}
}
