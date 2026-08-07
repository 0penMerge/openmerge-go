package openmerge

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) CreateLinkToken(ctx context.Context, workspaceID, endUserOriginID string, allowedCategories []string, hostOrigin string) (LinkToken, error) {
	var wire struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
		HostedURL string `json:"hosted_url"`
	}
	err := c.request(ctx, http.MethodPost, "/link-tokens", nil, map[string]any{
		"wsid": workspaceID, "end_user_origin_id": endUserOriginID,
		"allowed_categories": allowedCategories, "host_origin": hostOrigin,
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
