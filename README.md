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

Per-connection application-to-CRM defaults are passed server-side when the link
token is minted. Developer IR policy determines whether the customer can change
the selection in the widget:

```go
link, err := client.CreateLinkTokenWithOptions(ctx, "ws_123", "customer_456", openmerge.LinkTokenOptions{
    MappingOverrides: map[string]map[string]map[string]string{
        "salesforce": {"Contact": {"research_bio": "Research_Bio__c"}},
    },
})
```

The SDK keeps workspace keys server-side, retries only safe or idempotency-keyed requests, exposes structured API errors, and verifies raw webhook bodies:

```go
verified, err := openmerge.VerifyWebhookSignature(rawBody, signature, secret, 5*time.Minute, time.Now())
```

## Development

```bash
go fmt ./...
go vet ./...
go test -race ./...
```

## Codex and GPT-5.6

Codex and GPT-5.6 were used to audit the API contract, design retry/idempotency and webhook boundaries, generate the initial implementation, and build the test matrix. Maintainers remain responsible for review, release approval, and compatibility decisions.

Licensed under AGPL-3.0-only.
