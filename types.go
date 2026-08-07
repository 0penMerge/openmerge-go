package openmerge

type LinkToken struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"`
	HostedURL string `json:"hostedUrl"`
}

type LinkedAccount struct {
	ID                string   `json:"id"`
	WorkspaceID       string   `json:"wsid"`
	Provider          string   `json:"provider"`
	ProviderName      string   `json:"provider_display_name,omitempty"`
	EndUserOriginID   string   `json:"end_user_origin_id"`
	EndUserLabel      string   `json:"end_user_label,omitempty"`
	Status            string   `json:"status"`
	LastSyncAt        *float64 `json:"last_sync_at,omitempty"`
	SyncCount         int      `json:"sync_count,omitempty"`
	SyncCadenceSecond int      `json:"sync_cadence_seconds,omitempty"`
	CreatedAt         float64  `json:"created_at"`
}

type ConnectorDescriptor struct {
	ID           string         `json:"id"`
	Version      string         `json:"version,omitempty"`
	Name         string         `json:"name"`
	Company      string         `json:"company,omitempty"`
	Description  string         `json:"description,omitempty"`
	Logo         map[string]any `json:"logo,omitempty"`
	Categories   []string       `json:"categories,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Auth         map[string]any `json:"auth,omitempty"`
	Setup        map[string]any `json:"setup,omitempty"`
}

type ConnectorIntegration struct {
	Provider           string              `json:"provider"`
	Manifest           map[string]any      `json:"manifest,omitempty"`
	Descriptor         ConnectorDescriptor `json:"descriptor"`
	CallbackURL        string              `json:"callback_url,omitempty"`
	Configured         bool                `json:"configured,omitempty"`
	OAuthAppCount      int                 `json:"oauth_app_count,omitempty"`
	LinkedAccountCount int                 `json:"linked_account_count,omitempty"`
}

type UnifiedModelDefinition struct {
	ID                           string                    `json:"id"`
	Category                     string                    `json:"category,omitempty"`
	Version                      any                       `json:"version,omitempty"`
	Fields                       map[string]map[string]any `json:"fields,omitempty"`
	BaseTwoWayFields             int                       `json:"base_two_way_fields,omitempty"`
	BaseTwoWayFieldIDs           []string                  `json:"base_two_way_field_ids,omitempty"`
	BaseTwoWayFieldIDsByProvider map[string][]string       `json:"base_two_way_field_ids_by_provider,omitempty"`
	BaseMappedProviders          int                       `json:"base_mapped_providers,omitempty"`
	BaseMappedProviderIDs        []string                  `json:"base_mapped_provider_ids,omitempty"`
}

type UnifiedRecord struct {
	WorkspaceID     string         `json:"wsid"`
	LinkedAccountID string         `json:"linked_account_id"`
	Provider        string         `json:"provider"`
	Model           string         `json:"model"`
	UnifiedID       string         `json:"unified_id"`
	Data            map[string]any `json:"data"`
	RemoteData      any            `json:"remote_data,omitempty"`
	DeletedAt       *float64       `json:"deleted_at,omitempty"`
}

type UnifiedRecordPage struct {
	Records    []UnifiedRecord `json:"records"`
	NextCursor *string         `json:"next_cursor"`
}

type Writeback struct {
	ID              string `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	WorkspaceID     string `json:"wsid,omitempty"`
	LinkedAccountID string `json:"linked_account_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	UnifiedID       string `json:"unified_id,omitempty"`
	State           string `json:"state"`
	Result          any    `json:"result,omitempty"`
	ErrorType       string `json:"error_type,omitempty"`
}
