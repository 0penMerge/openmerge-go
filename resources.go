package openmerge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hostedWidgetTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,512}$`)
var modelIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,127}$`)

type linkTokenWire struct {
	Token          string `json:"token"`
	ExpiresIn      int    `json:"expires_in"`
	HostedURL      string `json:"hosted_url"`
	DiscoveryJobID string `json:"discovery_job_id,omitempty"`
}

func linkTokenFromWire(wire linkTokenWire) LinkToken {
	return LinkToken{
		Token:     wire.Token,
		ExpiresIn: wire.ExpiresIn,
		HostedURL: wire.HostedURL,
	}
}

func mappingLinkTokenFromWire(wire linkTokenWire) MappingLinkToken {
	return MappingLinkToken{
		Token:          wire.Token,
		ExpiresIn:      wire.ExpiresIn,
		HostedURL:      wire.HostedURL,
		DiscoveryJobID: wire.DiscoveryJobID,
	}
}

type hostedWidgetOrigin struct {
	scheme string
	host   string
	port   string
}

func originOf(value *url.URL) hostedWidgetOrigin {
	port := value.Port()
	if port == "" {
		switch strings.ToLower(value.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return hostedWidgetOrigin{
		scheme: strings.ToLower(value.Scheme),
		host:   strings.ToLower(value.Hostname()),
		port:   port,
	}
}

func isLoopbackWidgetHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func parseAllowedWidgetOrigin(value string) (hostedWidgetOrigin, error) {
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Hostname() == "" {
		return hostedWidgetOrigin{}, errors.New("openmerge: allowed widget origin must be absolute")
	}
	origin := originOf(parsed)
	if origin.scheme != "https" && !(origin.scheme == "http" && isLoopbackWidgetHost(origin.host)) {
		return hostedWidgetOrigin{}, errors.New("openmerge: allowed widget origin must use HTTPS outside loopback development")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return hostedWidgetOrigin{}, errors.New("openmerge: allowed widget origin cannot contain credentials, path, query, or fragment")
	}
	return origin, nil
}

func validHostedWidgetMode(mode HostedWidgetMode) bool {
	switch mode {
	case HostedWidgetConnect, HostedWidgetManager, HostedWidgetFieldMapping, HostedWidgetSyncStatus:
		return true
	default:
		return false
	}
}

// HostedWidgetURL derives a portable hosted surface from the URL returned by
// a token-minting API. It never accepts credentials, opaque origins, or
// cleartext non-loopback hosts. Official OpenMerge and loopback origins are
// accepted by default; self-hosted deployments must pass their exact HTTPS
// origin in allowedOrigins.
func HostedWidgetURL(hostedURL string, mode HostedWidgetMode, model string, allowedOrigins ...string) (string, error) {
	parsed, err := url.Parse(hostedURL)
	if err != nil || !parsed.IsAbs() || parsed.Hostname() == "" {
		return "", errors.New("openmerge: hosted widget URL must be absolute")
	}
	targetOrigin := originOf(parsed)
	loopback := isLoopbackWidgetHost(targetOrigin.host)
	if targetOrigin.scheme != "https" && !(targetOrigin.scheme == "http" && loopback) {
		return "", errors.New("openmerge: hosted widget URL must use HTTPS outside loopback development")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("openmerge: hosted widget URL cannot contain credentials or a fragment")
	}
	official := targetOrigin == (hostedWidgetOrigin{scheme: "https", host: "widgets.openmerge.dev", port: "443"})
	allowed := official || loopback
	for _, value := range allowedOrigins {
		origin, originErr := parseAllowedWidgetOrigin(value)
		if originErr != nil {
			return "", originErr
		}
		if origin == targetOrigin {
			allowed = true
		}
	}
	if !allowed {
		return "", errors.New("openmerge: hosted widget URL origin is not allowed")
	}
	segments := strings.Split(strings.TrimSuffix(parsed.Path, "/"), "/")
	if len(segments) != 3 || segments[0] != "" || segments[1] != "connect" || !hostedWidgetTokenPattern.MatchString(segments[2]) {
		return "", errors.New("openmerge: hosted widget URL does not contain a valid link token")
	}
	query := parsed.Query()
	existingModes := query["mode"]
	if len(existingModes) > 1 {
		return "", errors.New("openmerge: hosted widget URL contains multiple modes")
	}
	if len(existingModes) == 1 && !validHostedWidgetMode(HostedWidgetMode(existingModes[0])) {
		return "", errors.New("openmerge: hosted widget URL contains an unsupported mode")
	}
	if mode == "" {
		if len(existingModes) == 1 {
			mode = HostedWidgetMode(existingModes[0])
		} else {
			mode = HostedWidgetConnect
		}
	}
	if !validHostedWidgetMode(mode) {
		return "", errors.New("openmerge: unsupported hosted widget mode")
	}
	existingModels := query["model"]
	if len(existingModels) > 1 {
		return "", errors.New("openmerge: hosted widget URL contains multiple models")
	}
	if model == "" && len(existingModels) == 1 {
		model = existingModels[0]
	}
	if model != "" && !modelIDPattern.MatchString(model) {
		return "", errors.New("openmerge: model must be a valid OpenMerge model identifier")
	}
	query.Set("mode", string(mode))
	if model != "" {
		query.Set("model", model)
	} else {
		query.Del("model")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) CreateLinkToken(ctx context.Context, workspaceID, endUserOriginID string, allowedCategories []string, hostOrigin string) (LinkToken, error) {
	return c.CreateLinkTokenWithOptions(ctx, workspaceID, endUserOriginID, LinkTokenOptions{
		AllowedCategories: allowedCategories,
		HostOrigin:        hostOrigin,
	})
}

func (c *Client) CreateLinkTokenWithOptions(ctx context.Context, workspaceID, endUserOriginID string, options LinkTokenOptions) (LinkToken, error) {
	var wire linkTokenWire
	body := map[string]any{"wsid": workspaceID, "end_user_origin_id": endUserOriginID}
	if options.AllowedCategories != nil {
		body["allowed_categories"] = options.AllowedCategories
	}
	if options.HostOrigin != "" {
		body["host_origin"] = options.HostOrigin
	}
	if options.Appearance != nil {
		body["appearance"] = options.Appearance
	}
	if options.MappingOverrides != nil {
		body["mapping_overrides"] = options.MappingOverrides
	}
	err := c.request(ctx, http.MethodPost, "/link-tokens", nil, body, "", &wire)
	return linkTokenFromWire(wire), err
}

// CreateReconnectToken mints a hosted reconnect session for an existing
// linked account. The API preserves the account's provider and OAuth-app
// affinity rather than creating a second connection.
func (c *Client) CreateReconnectToken(ctx context.Context, workspaceID, linkedAccountID string, options ReconnectTokenOptions) (LinkToken, error) {
	var wire linkTokenWire
	body := map[string]any{"wsid": workspaceID, "linked_account_id": linkedAccountID}
	if options.EndUserOriginID != "" {
		body["end_user_origin_id"] = options.EndUserOriginID
	}
	if options.HostOrigin != "" {
		body["host_origin"] = options.HostOrigin
	}
	if options.Appearance != nil {
		body["appearance"] = options.Appearance
	}
	if options.MappingOverrides != nil {
		body["mapping_overrides"] = options.MappingOverrides
	}
	err := c.request(ctx, http.MethodPost, "/link-tokens", nil, body, "", &wire)
	return linkTokenFromWire(wire), err
}

func (c *Client) ListIntegrations(ctx context.Context, workspaceID string) ([]ConnectorIntegration, error) {
	var result struct {
		Integrations []ConnectorIntegration `json:"integrations"`
	}
	err := c.request(ctx, http.MethodGet, "/integrations", url.Values{"wsid": {workspaceID}}, nil, "", &result)
	return result.Integrations, err
}

func (c *Client) ListModels(ctx context.Context, workspaceID string) ([]UnifiedModelDefinition, error) {
	var result struct {
		Models []UnifiedModelDefinition `json:"models"`
	}
	query := url.Values{}
	if workspaceID != "" {
		query.Set("wsid", workspaceID)
	}
	err := c.request(ctx, http.MethodGet, "/models", query, nil, "", &result)
	return result.Models, err
}

func (c *Client) ListLinkedAccounts(ctx context.Context, workspaceID string) ([]LinkedAccount, error) {
	var result struct {
		LinkedAccounts []LinkedAccount `json:"linked_accounts"`
	}
	err := c.request(ctx, http.MethodGet, "/linked-accounts", url.Values{"wsid": {workspaceID}}, nil, "", &result)
	return result.LinkedAccounts, err
}

func (c *Client) GetLinkedAccount(ctx context.Context, linkedAccountID string) (LinkedAccount, error) {
	var result LinkedAccount
	err := c.request(ctx, http.MethodGet,
		"/linked-accounts/"+url.PathEscape(linkedAccountID), nil, nil, "", &result)
	return result, err
}

func (c *Client) PauseLinkedAccount(ctx context.Context, linkedAccountID string) (AccountTransition, error) {
	var result AccountTransition
	err := c.request(ctx, http.MethodPost,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/pause", nil, nil, "", &result)
	return result, err
}

func (c *Client) ResumeLinkedAccount(ctx context.Context, linkedAccountID string) (AccountTransition, error) {
	var result AccountTransition
	err := c.request(ctx, http.MethodPost,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/resume", nil, nil, "", &result)
	return result, err
}

func (c *Client) TriggerSync(ctx context.Context, linkedAccountID string, models []string) ([]map[string]any, error) {
	var result struct {
		Runs []map[string]any `json:"runs"`
	}
	err := c.request(ctx, http.MethodPost, "/linked-accounts/"+url.PathEscape(linkedAccountID)+"/sync", nil, map[string]any{"models": models}, "", &result)
	return result.Runs, err
}

func (c *Client) UpdateLinkedAccountSchedule(ctx context.Context, linkedAccountID string, cadenceSeconds int, options UpdateScheduleOptions) (AccountSchedule, error) {
	var result AccountSchedule
	err := c.request(ctx, http.MethodPut,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/schedule", nil, map[string]any{
			"cadence_seconds":                 cadenceSeconds,
			"reconciliation_seconds_by_model": options.ReconciliationSecondsByModel,
		}, "", &result)
	return result, err
}

func (c *Client) DeleteLinkedAccount(ctx context.Context, linkedAccountID string) (AccountDeletion, error) {
	var result AccountDeletion
	err := c.request(ctx, http.MethodDelete,
		"/linked-accounts/"+url.PathEscape(linkedAccountID), nil, nil, "", &result)
	return result, err
}

func (c *Client) GetDeveloperIR(ctx context.Context, workspaceID, oauthAppID, modelID string) (DeveloperIR, error) {
	var result DeveloperIR
	err := c.request(ctx, http.MethodGet,
		"/developer-ir/"+url.PathEscape(oauthAppID)+"/"+url.PathEscape(modelID),
		url.Values{"wsid": {workspaceID}}, nil, "", &result)
	return result, err
}

// ListDeveloperIRRevisions returns immutable generations newest-first. A zero
// limit uses the server default; explicit values must match the API's 1..100
// bound.
func (c *Client) ListDeveloperIRRevisions(ctx context.Context, workspaceID, oauthAppID, modelID string, limit int) ([]DeveloperIRRevision, error) {
	if limit < 0 || limit > 100 {
		return nil, errors.New("openmerge: developer IR revision limit must be between 1 and 100, or zero for the server default")
	}
	query := url.Values{"wsid": {workspaceID}}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var result struct {
		Revisions []DeveloperIRRevision `json:"revisions"`
	}
	err := c.request(ctx, http.MethodGet,
		"/developer-ir/"+url.PathEscape(oauthAppID)+"/"+url.PathEscape(modelID)+"/revisions",
		query, nil, "", &result)
	return result.Revisions, err
}

func (c *Client) UpdateDeveloperIR(ctx context.Context, workspaceID, oauthAppID, modelID string, expectedGeneration int, fields map[string]map[string]any, removedFields []string) (DeveloperIR, error) {
	return c.updateDeveloperIR(ctx, workspaceID, oauthAppID, modelID, expectedGeneration, fields, removedFields)
}

// UpdateDeveloperIRWithFields is the typed form of UpdateDeveloperIR. The
// original map-based method remains available for source compatibility and
// forward-compatible experimental contracts.
func (c *Client) UpdateDeveloperIRWithFields(ctx context.Context, workspaceID, oauthAppID, modelID string, expectedGeneration int, fields map[string]DeveloperIRField, removedFields []string) (DeveloperIR, error) {
	return c.updateDeveloperIR(ctx, workspaceID, oauthAppID, modelID, expectedGeneration, fields, removedFields)
}

func (c *Client) updateDeveloperIR(ctx context.Context, workspaceID, oauthAppID, modelID string, expectedGeneration int, fields any, removedFields []string) (DeveloperIR, error) {
	var result DeveloperIR
	err := c.request(ctx, http.MethodPut,
		"/developer-ir/"+url.PathEscape(oauthAppID)+"/"+url.PathEscape(modelID),
		url.Values{"wsid": {workspaceID}}, map[string]any{
			"expected_generation": expectedGeneration,
			"fields":              fields,
			"removed_fields":      removedFields,
		}, "", &result)
	return result, err
}

func (c *Client) GetConnectionMapping(ctx context.Context, linkedAccountID string) (ConnectionMappingSchema, error) {
	var result ConnectionMappingSchema
	err := c.request(ctx, http.MethodGet,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/field-mapping",
		nil, nil, "", &result)
	return result, err
}

func (c *Client) CreateConnectionMappingToken(ctx context.Context, linkedAccountID, hostOrigin string) (MappingLinkToken, error) {
	return c.CreateConnectionMappingTokenWithOptions(ctx, linkedAccountID, ConnectionMappingTokenOptions{
		HostOrigin: hostOrigin,
	})
}

func (c *Client) CreateConnectionMappingTokenWithOptions(ctx context.Context, linkedAccountID string, options ConnectionMappingTokenOptions) (MappingLinkToken, error) {
	if strings.TrimSpace(linkedAccountID) == "" {
		return MappingLinkToken{}, errors.New("openmerge: mapping token requires a linked account ID")
	}
	var wire linkTokenWire
	body := map[string]any{}
	if options.HostOrigin != "" {
		body["host_origin"] = options.HostOrigin
	}
	if options.Appearance != nil {
		body["appearance"] = options.Appearance
	}
	err := c.request(ctx, http.MethodPost,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/field-mapping-token",
		nil, body, "", &wire)
	if err != nil {
		return MappingLinkToken{}, err
	}
	if strings.TrimSpace(wire.DiscoveryJobID) == "" {
		return MappingLinkToken{}, errors.New("openmerge: mapping token response omitted discovery job ID")
	}
	return mappingLinkTokenFromWire(wire), nil
}

func (c *Client) ActivateConnectionMapping(ctx context.Context, linkedAccountID string, expectedDeveloperGenerations map[string]int, mappings map[string]map[string]string, idempotencyKey string) (ConnectionMappingJob, error) {
	var result ConnectionMappingJob
	err := c.request(ctx, http.MethodPost,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/field-mapping",
		nil, map[string]any{
			"expected_developer_generations": expectedDeveloperGenerations,
			"mappings":                       mappings,
			"idempotency_key":                idempotencyKey,
		}, "", &result)
	return result, err
}

func (c *Client) GetConnectionMappingJob(ctx context.Context, jobID string) (ConnectionMappingJob, error) {
	var result ConnectionMappingJob
	err := c.request(ctx, http.MethodGet,
		"/connection-mapping-jobs/"+url.PathEscape(jobID), nil, nil, "", &result)
	return result, err
}

func (c *Client) ListRecords(ctx context.Context, model, workspaceID, linkedAccountID, cursor string, pageSize int) (UnifiedRecordPage, error) {
	return c.ListRecordsWithOptions(ctx, model, workspaceID, linkedAccountID, ListRecordsOptions{
		Cursor: cursor, PageSize: pageSize,
	})
}

func (c *Client) ListRecordsWithOptions(ctx context.Context, model, workspaceID, linkedAccountID string, options ListRecordsOptions) (UnifiedRecordPage, error) {
	if options.PageSize == 0 {
		options.PageSize = 100
	}
	query := url.Values{
		"wsid": {workspaceID}, "linked_account_id": {linkedAccountID},
		"page_size": {strconv.Itoa(options.PageSize)},
	}
	if options.Cursor != "" {
		query.Set("cursor", options.Cursor)
	}
	if options.IncludeRemoteData {
		query.Set("include_remote_data", "true")
	}
	if options.IncludeDeleted {
		query.Set("include_deleted", "true")
	}
	var result UnifiedRecordPage
	err := c.request(ctx, http.MethodGet, "/unified/"+url.PathEscape(model), query, nil, "", &result)
	return result, err
}

// IterateRecords walks cursor pages without buffering the full result set.
// Returning an error from visit stops iteration immediately.
func (c *Client) IterateRecords(ctx context.Context, model, workspaceID, linkedAccountID string, options ListRecordsOptions, visit func(UnifiedRecord) error) error {
	if visit == nil {
		return errors.New("openmerge: record visitor must not be nil")
	}
	cursor := options.Cursor
	seen := make(map[string]struct{})
	if cursor != "" {
		seen[cursor] = struct{}{}
	}
	for {
		options.Cursor = cursor
		page, err := c.ListRecordsWithOptions(ctx, model, workspaceID, linkedAccountID, options)
		if err != nil {
			return err
		}
		for _, record := range page.Records {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := visit(record); err != nil {
				return err
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return nil
		}
		if _, exists := seen[*page.NextCursor]; exists {
			return errors.New("openmerge: record pagination returned a recurring cursor")
		}
		cursor = *page.NextCursor
		seen[cursor] = struct{}{}
	}
}

func validateIdempotencyKey(value string) error {
	if len(value) < 8 || len(value) > 200 || strings.TrimSpace(value) == "" {
		return errors.New("openmerge: idempotency key must be nonblank and contain 8 to 200 characters")
	}
	return nil
}

func validateRecordActionScope(scope RecordActionScope) error {
	if err := validateIdempotencyKey(scope.IdempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(scope.WorkspaceID) == "" || strings.TrimSpace(scope.LinkedAccountID) == "" {
		return errors.New("openmerge: record action requires workspace, linked account, and model")
	}
	if !modelIDPattern.MatchString(scope.Model) {
		return errors.New("openmerge: record action model must be a valid OpenMerge model identifier")
	}
	return nil
}

func (c *Client) submitRecordAction(ctx context.Context, scope RecordActionScope, operation string, actionBody map[string]any) (Writeback, error) {
	if err := validateRecordActionScope(scope); err != nil {
		return Writeback{}, err
	}
	body := map[string]any{
		"wsid":              scope.WorkspaceID,
		"linked_account_id": scope.LinkedAccountID,
	}
	for key, value := range actionBody {
		body[key] = value
	}
	var result struct {
		Write Writeback `json:"write"`
	}
	err := c.request(ctx, http.MethodPost,
		"/unified/"+url.PathEscape(scope.Model)+"/actions/"+operation,
		nil, body, scope.IdempotencyKey, &result)
	return result.Write, err
}

// CreateRecord creates a provider record through the active connection IR.
func (c *Client) CreateRecord(ctx context.Context, params CreateRecordParams) (Writeback, error) {
	if len(params.Changes) == 0 {
		return Writeback{}, errors.New("openmerge: create record changes must not be empty")
	}
	return c.submitRecordAction(ctx, params.RecordActionScope, "create", map[string]any{
		"changes": params.Changes,
	})
}

// UpsertRecord resolves an optional unified identity and creates or updates the
// corresponding provider record atomically under the supplied idempotency key.
func (c *Client) UpsertRecord(ctx context.Context, params UpsertRecordParams) (Writeback, error) {
	if len(params.Changes) == 0 {
		return Writeback{}, errors.New("openmerge: upsert record changes must not be empty")
	}
	if params.UnifiedID != "" && strings.TrimSpace(params.UnifiedID) == "" {
		return Writeback{}, errors.New("openmerge: upsert record unified ID must not be blank")
	}
	body := map[string]any{"changes": params.Changes}
	if params.UnifiedID != "" {
		body["unified_id"] = params.UnifiedID
	}
	return c.submitRecordAction(ctx, params.RecordActionScope, "upsert", body)
}

// DeleteRecord deletes the provider record bound to UnifiedID.
func (c *Client) DeleteRecord(ctx context.Context, params DeleteRecordParams) (Writeback, error) {
	if strings.TrimSpace(params.UnifiedID) == "" {
		return Writeback{}, errors.New("openmerge: delete record requires a unified ID")
	}
	return c.submitRecordAction(ctx, params.RecordActionScope, "delete", map[string]any{
		"unified_id": params.UnifiedID,
	})
}

// BulkRecords submits 1..100 operation-homogeneous create, update, upsert, or
// delete actions as one durable writeback.
func (c *Client) BulkRecords(ctx context.Context, params BulkRecordActionParams) (Writeback, error) {
	if err := validateRecordActionScope(params.RecordActionScope); err != nil {
		return Writeback{}, err
	}
	if len(params.Items) < 1 || len(params.Items) > 100 {
		return Writeback{}, errors.New("openmerge: bulk record actions require 1 to 100 items")
	}
	items := make([]BulkRecordActionItem, len(params.Items))
	operation := params.Items[0].Operation
	for index, item := range params.Items {
		switch item.Operation {
		case BulkRecordCreate, BulkRecordUpdate, BulkRecordUpsert, BulkRecordDelete:
		default:
			return Writeback{}, fmt.Errorf("openmerge: bulk record item %d has unsupported operation %q", index, item.Operation)
		}
		if item.Operation != operation {
			return Writeback{}, errors.New("openmerge: bulk record actions must use one operation per batch")
		}
		if item.UnifiedID != "" && strings.TrimSpace(item.UnifiedID) == "" {
			return Writeback{}, fmt.Errorf("openmerge: bulk record item %d unified ID must not be blank", index)
		}
		if item.RemoteID != "" && strings.TrimSpace(item.RemoteID) == "" {
			return Writeback{}, fmt.Errorf("openmerge: bulk record item %d remote ID must not be blank", index)
		}
		identities := 0
		if item.UnifiedID != "" {
			identities++
		}
		if item.RemoteID != "" {
			identities++
		}
		if identities > 1 {
			return Writeback{}, fmt.Errorf("openmerge: bulk record item %d must not specify both unified and remote IDs", index)
		}
		switch item.Operation {
		case BulkRecordCreate:
			if identities != 0 {
				return Writeback{}, fmt.Errorf("openmerge: bulk create item %d must not specify an existing identity", index)
			}
			if len(item.Changes) == 0 {
				return Writeback{}, fmt.Errorf("openmerge: bulk create item %d changes must not be empty", index)
			}
		case BulkRecordUpdate:
			if identities != 1 {
				return Writeback{}, fmt.Errorf("openmerge: bulk update item %d requires one unified or remote ID", index)
			}
			if len(item.Changes) == 0 {
				return Writeback{}, fmt.Errorf("openmerge: bulk update item %d changes must not be empty", index)
			}
		case BulkRecordUpsert:
			if len(item.Changes) == 0 {
				return Writeback{}, fmt.Errorf("openmerge: bulk upsert item %d changes must not be empty", index)
			}
		case BulkRecordDelete:
			if identities != 1 {
				return Writeback{}, fmt.Errorf("openmerge: bulk delete item %d requires one unified or remote ID", index)
			}
			if len(item.Changes) != 0 {
				return Writeback{}, fmt.Errorf("openmerge: bulk delete item %d must not include changes", index)
			}
		}
		if item.Changes == nil {
			item.Changes = map[string]any{}
		}
		items[index] = item
	}
	return c.submitRecordAction(ctx, params.RecordActionScope, "bulk", map[string]any{
		"items": items,
	})
}

// CreateCustomField provisions a provider-native custom field through the
// connector's schema-management capability.
func (c *Client) CreateCustomField(ctx context.Context, params CreateCustomFieldParams) (Writeback, error) {
	if len(params.Definition) == 0 {
		return Writeback{}, errors.New("openmerge: custom field definition must not be empty")
	}
	return c.submitRecordAction(ctx, params.RecordActionScope, "custom-fields", map[string]any{
		"definition": params.Definition,
	})
}

func (c *Client) SubmitWriteback(ctx context.Context, model, unifiedID, workspaceID, linkedAccountID string, changes map[string]any, idempotencyKey string) (Writeback, error) {
	if err := validateIdempotencyKey(idempotencyKey); err != nil {
		return Writeback{}, err
	}
	if len(changes) == 0 {
		return Writeback{}, errors.New("openmerge: changes must not be empty")
	}
	var result struct {
		Write Writeback `json:"write"`
	}
	err := c.request(ctx, http.MethodPost, "/unified/"+url.PathEscape(model)+"/"+url.PathEscape(unifiedID), nil, map[string]any{
		"wsid": workspaceID, "linked_account_id": linkedAccountID, "changes": changes,
	}, idempotencyKey, &result)
	return result.Write, err
}

func (c *Client) GetWriteback(ctx context.Context, writeID, workspaceID string) (Writeback, error) {
	var result struct {
		Write Writeback `json:"write"`
	}
	err := c.request(ctx, http.MethodGet, "/unified/writes/"+url.PathEscape(writeID), url.Values{"wsid": {workspaceID}}, nil, "", &result)
	return result.Write, err
}

func (c *Client) WaitForWriteback(ctx context.Context, writeID, workspaceID string, options WaitForWritebackOptions) (Writeback, error) {
	if options.Interval == 0 {
		options.Interval = time.Second
	}
	if options.Timeout == 0 {
		options.Timeout = time.Minute
	}
	if options.Interval < 10*time.Millisecond {
		return Writeback{}, errors.New("openmerge: writeback polling interval must be at least 10ms")
	}
	if options.Timeout < time.Millisecond {
		return Writeback{}, errors.New("openmerge: writeback polling timeout must be positive")
	}
	terminal := options.TerminalStates
	if terminal == nil {
		terminal = []string{"completed", "failed", "dispatch_failed"}
	}
	if len(terminal) == 0 {
		return Writeback{}, errors.New("openmerge: terminal states must not be empty")
	}
	terminalStates := make(map[string]struct{}, len(terminal))
	for _, state := range terminal {
		terminalStates[state] = struct{}{}
	}
	pollCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	timeoutError := func() error {
		return &RequestTimeoutError{WriteID: writeID, Timeout: options.Timeout}
	}
	for {
		if err := pollCtx.Err(); err != nil {
			if ctx.Err() != nil {
				return Writeback{}, ctx.Err()
			}
			return Writeback{}, timeoutError()
		}
		write, err := c.GetWriteback(pollCtx, writeID, workspaceID)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return Writeback{}, timeoutError()
			}
			return Writeback{}, err
		}
		if _, done := terminalStates[write.State]; done {
			return write, nil
		}
		deadline, _ := pollCtx.Deadline()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return Writeback{}, timeoutError()
		}
		delay := options.Interval
		if remaining < delay {
			delay = remaining
		}
		if err := wait(pollCtx, delay); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return Writeback{}, timeoutError()
			}
			return Writeback{}, err
		}
	}
}

func mailQuery(scope MailScope) url.Values {
	return url.Values{"wsid": {scope.WorkspaceID}, "end_user_origin_id": {scope.OwnerID}}
}
func (c *Client) ListMailMessages(ctx context.Context, scope MailScope, view, cursor string, limit int) (MailRecordPage, error) {
	query := mailQuery(scope)
	if view != "" {
		query.Set("view", view)
	}
	query.Set("cursor", cursor)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var result MailRecordPage
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/messages", query, nil, "", &result)
	return result, err
}
func (c *Client) SendMail(ctx context.Context, scope MailScope, message MailSendRequest, idempotencyKey string) (MailOperation, error) {
	var result MailOperation
	err := c.request(ctx, http.MethodPost, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/send", mailQuery(scope), message, idempotencyKey, &result)
	return result, err
}
func (c *Client) GetMailOperation(ctx context.Context, scope MailScope, operationID string) (MailOperation, error) {
	var result MailOperation
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/operations/"+url.PathEscape(operationID), mailQuery(scope), nil, "", &result)
	return result, err
}
func (c *Client) GetMailContent(ctx context.Context, scope MailScope, messageID, kind string) ([]byte, error) {
	query := mailQuery(scope)
	if kind != "" {
		query.Set("kind", kind)
	}
	var result []byte
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/messages/"+url.PathEscape(messageID)+"/content", query, nil, "", &result)
	return result, err
}

func (c *Client) ListMailContainers(ctx context.Context, scope MailScope, cursor string) (MailRecordPage, error) {
	query := mailQuery(scope)
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var result MailRecordPage
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/containers", query, nil, "", &result)
	return result, err
}
func (c *Client) GetMailMessage(ctx context.Context, scope MailScope, messageID string) (UnifiedRecord, error) {
	var result UnifiedRecord
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/messages/"+url.PathEscape(messageID), mailQuery(scope), nil, "", &result)
	return result, err
}
func (c *Client) GetMailAttachment(ctx context.Context, scope MailScope, messageID, part string) ([]byte, error) {
	var result []byte
	err := c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/messages/"+url.PathEscape(messageID)+"/attachments/"+url.PathEscape(part), mailQuery(scope), nil, "", &result)
	return result, err
}
func (c *Client) UploadMailContent(ctx context.Context, scope MailScope, content io.Reader, contentType string) (MailBlob, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var result MailBlob
	err := c.request(ctx, http.MethodPost, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/content", mailQuery(scope), mailUpload{content, contentType}, "", &result)
	return result, err
}
func (c *Client) DownloadMailContent(ctx context.Context, scope MailScope, messageID, kind string, destination io.Writer) error {
	query := mailQuery(scope)
	if kind != "" {
		query.Set("kind", kind)
	}
	return c.request(ctx, http.MethodGet, "/mail/accounts/"+url.PathEscape(scope.AccountID)+"/messages/"+url.PathEscape(messageID)+"/content", query, nil, "", destination)
}
