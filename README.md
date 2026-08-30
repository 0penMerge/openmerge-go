# OpenMerge Go

Typed Go client for OpenMerge's unified integration API.

## Setup

```bash
go get github.com/0penMerge/openmerge-go
```

```go
client, err := openmerge.NewClient(openmerge.Config{
    APIKey: os.Getenv("OPENMERGE_API_KEY"),
})
integrations, err := client.ListIntegrations(ctx, "ws_123")
models, err := client.ListModels(ctx, "ws_123")
accounts, err := client.ListLinkedAccounts(ctx, "ws_123")
```

Keep workspace API keys on your backend. Only the short-lived token or hosted
URL may cross into an end-user application.

## Hosted and embedded widgets

Mint a connection session with application-to-CRM mapping defaults and typed
design tokens. `HostOrigin` must be the exact iframe host origin. Omit it for a
top-level hosted link or a native/desktop WebView bridge.

```go
radius := 12.0
link, err := client.CreateLinkTokenWithOptions(ctx, "ws_123", "customer_456", openmerge.LinkTokenOptions{
    AllowedCategories: []string{"crm"},
    HostOrigin: "https://app.customer.example",
    Appearance: &openmerge.WidgetAppearance{
        Mode: openmerge.WidgetColorModeDark,
        Colors: &openmerge.WidgetColors{
            Primary: "#84cc16",
            SurfaceHover: "#18181b",
            BorderStrong: "#52525b",
        },
        Shape: &openmerge.WidgetShape{Radius: &radius},
        Branding: &openmerge.WidgetBranding{
            ProductName: "Acme CRM",
            LogoURL: "https://cdn.customer.example/logo.svg",
        },
    },
    MappingOverrides: map[string]map[string]map[string]string{
        "salesforce": {"Contact": {"research_bio": "Research_Bio__c"}},
    },
})

// Return link.HostedURL directly when no frontend SDK can be installed.
// Other hosted surfaces are derived without exposing the workspace key.
managerURL, err := openmerge.HostedWidgetURL(
    link.HostedURL,
    openmerge.HostedWidgetManager,
    "",
)
```

The supported hosted modes are `connect`, `manager`, `field-mapping`, and
`sync-status`. Passing an empty mode preserves a valid mode already issued by
the server, or defaults to `connect` when none exists. Field mapping accepts an
optional model identifier:

```go
mappingURL, err := openmerge.HostedWidgetURL(
    link.HostedURL,
    openmerge.HostedWidgetFieldMapping,
    "Contact",
)
```

For a self-hosted widget deployment, explicitly allow its exact HTTPS origin;
the three-argument form remains unchanged for the official
`https://widgets.openmerge.dev` origin and loopback development:

```go
selfHostedURL, err := openmerge.HostedWidgetURL(
    link.HostedURL,
    openmerge.HostedWidgetManager,
    "",
    "https://widgets.customer.example",
)
```

Reconnect keeps the existing provider and OAuth-app affinity:

```go
reconnect, err := client.CreateReconnectToken(ctx, "ws_123", "la_123", openmerge.ReconnectTokenOptions{
    EndUserOriginID: "customer_456",
    HostOrigin: "https://app.customer.example",
})
```

To reopen mapping separately, mint a mapping token. It returns a distinct
`MappingLinkToken`; its hosted URL is usable without a frontend SDK and its
required `DiscoveryJobID` identifies the schema-discovery job.

```go
mapping, err := client.CreateConnectionMappingTokenWithOptions(ctx, "la_123", openmerge.ConnectionMappingTokenOptions{
    HostOrigin: "https://app.customer.example",
    Appearance: &openmerge.WidgetAppearance{Density: openmerge.WidgetDensityCompact},
})
```

Developer IR fields have a typed contract while the original map-based update
method remains source-compatible:

```go
nullable := false
ir, err := client.UpdateDeveloperIRWithFields(ctx, "ws_123", "oauth_app_123", "Contact", generation, map[string]openmerge.DeveloperIRField{
    "research_bio": {
        Type: openmerge.DeveloperFieldString,
        Direction: openmerge.DeveloperFieldTwoWay,
        Nullable: &nullable,
        OverrideBy: []openmerge.DeveloperFieldOverride{
            openmerge.DeveloperFieldOverrideApplication,
            openmerge.DeveloperFieldOverrideCustomer,
        },
        Sensitivity: openmerge.DeveloperFieldSensitivityPII,
    },
}, nil)

// ir.BaseDocument is the immutable shipped model. Revision history is ordered
// newest-first and can be used for audit or rollback tooling.
revisions, err := client.ListDeveloperIRRevisions(
    ctx, "ws_123", "oauth_app_123", "Contact", 50,
)
```

## Account lifecycle and records

```go
account, err := client.GetLinkedAccount(ctx, "la_123")
paused, err := client.PauseLinkedAccount(ctx, account.ID)
resumed, err := client.ResumeLinkedAccount(ctx, account.ID)
schedule, err := client.UpdateLinkedAccountSchedule(ctx, account.ID, 3600, openmerge.UpdateScheduleOptions{
    ReconciliationSecondsByModel: map[string]int{"Contact": 86400},
})

err = client.IterateRecords(ctx, "Contact", "ws_123", account.ID, openmerge.ListRecordsOptions{
    PageSize: 100,
    IncludeRemoteData: false,
    IncludeDeleted: true,
}, func(record openmerge.UnifiedRecord) error {
    return persist(record)
})
```

`ListRecords` remains available for backward compatibility;
`ListRecordsWithOptions` adds remote-data and deletion filters. Deleting a
linked account returns the durable erasure job identifier.

## Idempotent writeback

```go
write, err := client.SubmitWriteback(
    ctx,
    "Contact",
    "contact_123",
    "ws_123",
    "la_123",
    map[string]any{"email": "new@example.com"},
    "customer-event-123",
)
terminal, err := client.WaitForWriteback(ctx, write.ID, "ws_123", openmerge.WaitForWritebackOptions{
    Timeout: 2 * time.Minute,
})
```

The same durable writeback channel exposes create, identity-aware upsert,
delete, validated bulk operations, and provider-native custom-field
provisioning:

```go
scope := openmerge.RecordActionScope{
    WorkspaceID: "ws_123",
    LinkedAccountID: "la_123",
    Model: "Contact",
    IdempotencyKey: "customer-event-124",
}
created, err := client.CreateRecord(ctx, openmerge.CreateRecordParams{
    RecordActionScope: scope,
    Changes: map[string]any{"email": "person@example.com"},
})

scope.IdempotencyKey = "customer-batch-125"
batch, err := client.BulkRecords(ctx, openmerge.BulkRecordActionParams{
    RecordActionScope: scope,
    Items: []openmerge.BulkRecordActionItem{
        {Operation: openmerge.BulkRecordUpsert, UnifiedID: "contact_123", Changes: map[string]any{"status": "qualified"}},
        {Operation: openmerge.BulkRecordUpsert, RemoteID: "003xx0000000456", Changes: map[string]any{"status": "qualified"}},
    },
})
```

Bulk requests accept 1–100 operation-homogeneous items. Create/update/upsert
items require changes; update/delete require exactly one unified or remote ID.
Every action requires an 8–200 character idempotency key and returns a
`Writeback` that can be passed to `WaitForWriteback`.

Polling honors context cancellation/deadlines and supports custom terminal
states. The SDK retries only safe or explicitly idempotent requests and exposes
structured API errors.

## Webhook verification

```go
verified, err := openmerge.VerifyWebhookSignature(rawBody, signature, secret, 5*time.Minute, time.Now())
var event openmerge.RecordWebhookEnvelope
err = json.Unmarshal(verified.Payload, &event)

if event.Event == openmerge.RecordWebhookEventDeleted {
    // Apply the deletion using event.Data and event.Breadcrumbs.
}
```

Always pass the raw body bytes; parsing and re-encoding JSON changes the signed
payload. Use `RecordWebhookEnvelope` for `record.*` CDC deliveries and
`DomainWebhookEnvelope` for sync, connection, mapping, and lifecycle events.
Both expose workspace/user/integration/OAuth-app/connection context; their
typed breadcrumbs match their distinct event sources. `WebhookEnvelope`
remains as a backward-compatible, dynamically-typed record envelope.

## Development

```bash
go fmt ./...
go vet ./...
go test -race ./...
```

## Codex and GPT-5.6

Codex and GPT-5.6 were used to audit the API contract, design retry/idempotency and webhook boundaries, generate the initial implementation, and build the test matrix. Maintainers remain responsible for review, release approval, and compatibility decisions.

Licensed under AGPL-3.0-only.
