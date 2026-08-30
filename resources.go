package openmerge

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) CreateLinkToken(ctx context.Context, workspaceID, endUserOriginID string, allowedCategories []string, hostOrigin string) (LinkToken, error) {
	return c.CreateLinkTokenWithOptions(ctx, workspaceID, endUserOriginID, LinkTokenOptions{
		AllowedCategories: allowedCategories,
		HostOrigin:        hostOrigin,
	})
}

func (c *Client) CreateLinkTokenWithOptions(ctx context.Context, workspaceID, endUserOriginID string, options LinkTokenOptions) (LinkToken, error) {
	var wire struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
		HostedURL string `json:"hosted_url"`
	}
	err := c.request(ctx, http.MethodPost, "/link-tokens", nil, map[string]any{
		"wsid": workspaceID, "end_user_origin_id": endUserOriginID,
		"allowed_categories": options.AllowedCategories, "host_origin": options.HostOrigin,
		"mapping_overrides": options.MappingOverrides,
	}, "", &wire)
	return LinkToken{Token: wire.Token, ExpiresIn: wire.ExpiresIn, HostedURL: wire.HostedURL}, err
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

func (c *Client) TriggerSync(ctx context.Context, linkedAccountID string, models []string) ([]map[string]any, error) {
	var result struct {
		Runs []map[string]any `json:"runs"`
	}
	err := c.request(ctx, http.MethodPost, "/linked-accounts/"+url.PathEscape(linkedAccountID)+"/sync", nil, map[string]any{"models": models}, "", &result)
	return result.Runs, err
}

func (c *Client) GetDeveloperIR(ctx context.Context, workspaceID, oauthAppID, modelID string) (DeveloperIR, error) {
	var result DeveloperIR
	err := c.request(ctx, http.MethodGet,
		"/developer-ir/"+url.PathEscape(oauthAppID)+"/"+url.PathEscape(modelID),
		url.Values{"wsid": {workspaceID}}, nil, "", &result)
	return result, err
}

func (c *Client) UpdateDeveloperIR(ctx context.Context, workspaceID, oauthAppID, modelID string, expectedGeneration int, fields map[string]map[string]any, removedFields []string) (DeveloperIR, error) {
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

func (c *Client) CreateConnectionMappingToken(ctx context.Context, linkedAccountID, hostOrigin string) (LinkToken, error) {
	var wire struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
		HostedURL string `json:"hosted_url"`
	}
	err := c.request(ctx, http.MethodPost,
		"/linked-accounts/"+url.PathEscape(linkedAccountID)+"/field-mapping-token",
		nil, map[string]any{"host_origin": hostOrigin}, "", &wire)
	return LinkToken{Token: wire.Token, ExpiresIn: wire.ExpiresIn, HostedURL: wire.HostedURL}, err
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
	if pageSize == 0 {
		pageSize = 100
	}
	query := url.Values{
		"wsid": {workspaceID}, "linked_account_id": {linkedAccountID},
		"page_size": {strconv.Itoa(pageSize)},
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var result UnifiedRecordPage
	err := c.request(ctx, http.MethodGet, "/unified/"+url.PathEscape(model), query, nil, "", &result)
	return result, err
}

func (c *Client) SubmitWriteback(ctx context.Context, model, unifiedID, workspaceID, linkedAccountID string, changes map[string]any, idempotencyKey string) (Writeback, error) {
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 {
		return Writeback{}, errors.New("openmerge: idempotency key must contain 8 to 200 characters")
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
