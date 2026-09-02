package openmerge

import "time"

type LinkToken struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"`
	HostedURL string `json:"hostedUrl"`
}

// MappingLinkToken identifies both the hosted mapping session and the durable
// schema-discovery job that must finish before the mapping can be activated.
type MappingLinkToken struct {
	Token          string `json:"token"`
	ExpiresIn      int    `json:"expiresIn"`
	HostedURL      string `json:"hostedUrl"`
	DiscoveryJobID string `json:"discoveryJobId"`
}

type HostedWidgetMode string

const (
	HostedWidgetConnect      HostedWidgetMode = "connect"
	HostedWidgetManager      HostedWidgetMode = "manager"
	HostedWidgetFieldMapping HostedWidgetMode = "field-mapping"
	HostedWidgetSyncStatus   HostedWidgetMode = "sync-status"
)

type WidgetColorMode string

const (
	WidgetColorModeLight  WidgetColorMode = "light"
	WidgetColorModeDark   WidgetColorMode = "dark"
	WidgetColorModeSystem WidgetColorMode = "system"
)

type WidgetDensity string

const (
	WidgetDensityCompact     WidgetDensity = "compact"
	WidgetDensityComfortable WidgetDensity = "comfortable"
)

// WidgetAppearance is the portable, server-issued widget theme contract.
// OpenMerge validates these values when the link token is minted; arbitrary
// CSS and HTML are intentionally not accepted.
type WidgetAppearance struct {
	Mode       WidgetColorMode   `json:"mode,omitempty"`
	Density    WidgetDensity     `json:"density,omitempty"`
	Colors     *WidgetColors     `json:"colors,omitempty"`
	Typography *WidgetTypography `json:"typography,omitempty"`
	Shape      *WidgetShape      `json:"shape,omitempty"`
	Branding   *WidgetBranding   `json:"branding,omitempty"`
}

type WidgetColors struct {
	Primary      string `json:"primary,omitempty"`
	Background   string `json:"background,omitempty"`
	Panel        string `json:"panel,omitempty"`
	Surface      string `json:"surface,omitempty"`
	SurfaceHover string `json:"surfaceHover,omitempty"`
	Border       string `json:"border,omitempty"`
	BorderStrong string `json:"borderStrong,omitempty"`
	Text         string `json:"text,omitempty"`
	Muted        string `json:"muted,omitempty"`
	Danger       string `json:"danger,omitempty"`
}

type WidgetTypography struct {
	FontFamily string   `json:"fontFamily,omitempty"`
	Scale      *float64 `json:"scale,omitempty"`
}

type WidgetShape struct {
	Radius *float64 `json:"radius,omitempty"`
}

type WidgetBranding struct {
	ProductName string `json:"productName,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
}

type LinkTokenOptions struct {
	AllowedCategories []string                                `json:"allowed_categories,omitempty"`
	HostOrigin        string                                  `json:"host_origin,omitempty"`
	Appearance        *WidgetAppearance                       `json:"appearance,omitempty"`
	MappingOverrides  map[string]map[string]map[string]string `json:"mapping_overrides,omitempty"`
}

type ReconnectTokenOptions struct {
	EndUserOriginID  string                                  `json:"end_user_origin_id,omitempty"`
	HostOrigin       string                                  `json:"host_origin,omitempty"`
	Appearance       *WidgetAppearance                       `json:"appearance,omitempty"`
	MappingOverrides map[string]map[string]map[string]string `json:"mapping_overrides,omitempty"`
}

type ConnectionMappingTokenOptions struct {
	HostOrigin string            `json:"host_origin,omitempty"`
	Appearance *WidgetAppearance `json:"appearance,omitempty"`
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
	MappingStatus     string   `json:"mapping_status,omitempty"`
	MappingRequired   bool     `json:"mapping_required,omitempty"`
}

type SyncRun struct {
	ID         string `json:"id"`
	WorkflowID string `json:"workflow_id"`
	State      string `json:"state"`
}

type AccountTransition struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	CancelledRuns *int      `json:"cancelled_runs,omitempty"`
	Runs          []SyncRun `json:"runs,omitempty"`
}

type AccountSchedule struct {
	ID                           string         `json:"id"`
	CadenceSeconds               int            `json:"cadence_seconds"`
	ScheduledModels              []string       `json:"scheduled_models"`
	ReconciliationSecondsByModel map[string]int `json:"reconciliation_seconds_by_model,omitempty"`
}

type UpdateScheduleOptions struct {
	ReconciliationSecondsByModel map[string]int
}

type AccountDeletion struct {
	Deleted       bool   `json:"deleted"`
	CancelledRuns int    `json:"cancelled_runs"`
	ErasureJobID  string `json:"erasure_job_id"`
	ErasureState  string `json:"erasure_state"`
}

type DeveloperIR struct {
	WorkspaceID   string                    `json:"wsid"`
	OAuthAppID    string                    `json:"oauth_app_id"`
	Provider      string                    `json:"provider,omitempty"`
	ModelID       string                    `json:"model_id"`
	Generation    int                       `json:"generation"`
	BaseHash      string                    `json:"base_hash,omitempty"`
	DocumentHash  string                    `json:"document_hash"`
	ContractHash  string                    `json:"contract_hash,omitempty"`
	Inherited     bool                      `json:"inherited"`
	Document      map[string]any            `json:"document"`
	BaseDocument  map[string]any            `json:"base_document,omitempty"`
	Requirements  map[string]map[string]any `json:"requirements"`
	RemovedFields []string                  `json:"removed_fields"`
	ActorID       string                    `json:"actor_id,omitempty"`
	CreatedAt     *float64                  `json:"created_at,omitempty"`
	UpdatedAt     *float64                  `json:"updated_at,omitempty"`
}

// DeveloperIRRevision is an immutable Developer IR generation returned by
// ListDeveloperIRRevisions. HeadID identifies the logical revision stream;
// callers should use Generation for ordering and optimistic concurrency.
type DeveloperIRRevision struct {
	HeadID        string                    `json:"head_id,omitempty"`
	WorkspaceID   string                    `json:"wsid"`
	Provider      string                    `json:"provider"`
	OAuthAppID    string                    `json:"oauth_app_id"`
	ModelID       string                    `json:"model_id"`
	Generation    int                       `json:"generation"`
	BaseHash      string                    `json:"base_hash"`
	DocumentHash  string                    `json:"document_hash"`
	ContractHash  string                    `json:"contract_hash"`
	Document      map[string]any            `json:"document"`
	Requirements  map[string]map[string]any `json:"requirements"`
	RemovedFields []string                  `json:"removed_fields"`
	ActorID       string                    `json:"actor_id"`
	CreatedAt     float64                   `json:"created_at"`
}

type DeveloperFieldType string

const (
	DeveloperFieldString   DeveloperFieldType = "string"
	DeveloperFieldNumber   DeveloperFieldType = "number"
	DeveloperFieldInteger  DeveloperFieldType = "integer"
	DeveloperFieldBoolean  DeveloperFieldType = "boolean"
	DeveloperFieldDateTime DeveloperFieldType = "datetime"
	DeveloperFieldDate     DeveloperFieldType = "date"
	DeveloperFieldObject   DeveloperFieldType = "object"
	DeveloperFieldArray    DeveloperFieldType = "array"
	DeveloperFieldEnum     DeveloperFieldType = "enum"
)

type DeveloperFieldDirection string

const (
	DeveloperFieldReadOnly  DeveloperFieldDirection = "read_only"
	DeveloperFieldWriteOnly DeveloperFieldDirection = "write_only"
	DeveloperFieldTwoWay    DeveloperFieldDirection = "two_way"
)

type DeveloperFieldOverride string

const (
	DeveloperFieldOverrideApplication DeveloperFieldOverride = "application"
	DeveloperFieldOverrideCustomer    DeveloperFieldOverride = "customer"
)

type DeveloperFieldSensitivity string

const (
	DeveloperFieldSensitivityNone   DeveloperFieldSensitivity = "none"
	DeveloperFieldSensitivityPII    DeveloperFieldSensitivity = "pii"
	DeveloperFieldSensitivitySecret DeveloperFieldSensitivity = "secret"
)

// DeveloperIRField describes one application-side field contract. Items stays
// open so nested JSON-schema fragments survive round trips as the model catalog
// evolves.
type DeveloperIRField struct {
	Type                 DeveloperFieldType        `json:"type,omitempty"`
	Label                string                    `json:"label,omitempty"`
	Required             bool                      `json:"required,omitempty"`
	Direction            DeveloperFieldDirection   `json:"direction,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Nullable             *bool                     `json:"nullable,omitempty"`
	DefaultProviderField string                    `json:"default_provider_field,omitempty"`
	OverrideBy           []DeveloperFieldOverride  `json:"override_by,omitempty"`
	Items                map[string]any            `json:"items,omitempty"`
	Values               []string                  `json:"values,omitempty"`
	Ref                  string                    `json:"ref,omitempty"`
	Sensitivity          DeveloperFieldSensitivity `json:"sensitivity,omitempty"`
}

type ConnectionMappingField struct {
	ID                     string                    `json:"id"`
	Label                  string                    `json:"label"`
	Type                   DeveloperFieldType        `json:"type,omitempty"`
	Required               bool                      `json:"required"`
	RequirementSource      string                    `json:"requirement_source"`
	ProviderCreateRequired bool                      `json:"provider_create_required"`
	Direction              DeveloperFieldDirection   `json:"direction"`
	DefaultProviderField   string                    `json:"default_provider_field,omitempty"`
	OverrideBy             []DeveloperFieldOverride  `json:"override_by"`
	ApplicationOverridable bool                      `json:"application_overridable"`
	CustomerEditable       bool                      `json:"customer_editable"`
	MappedTo               string                    `json:"mapped_to,omitempty"`
	BindingSource          string                    `json:"binding_source"`
	BindingError           string                    `json:"binding_error,omitempty"`
	Format                 string                    `json:"format,omitempty"`
	Nullable               *bool                     `json:"nullable,omitempty"`
	Items                  map[string]any            `json:"items,omitempty"`
	Values                 []string                  `json:"values,omitempty"`
	Ref                    string                    `json:"ref,omitempty"`
	Sensitivity            DeveloperFieldSensitivity `json:"sensitivity,omitempty"`
	Description            string                    `json:"description,omitempty"`
}

type ConnectionMappingProviderCreateRequirement struct {
	Path      string   `json:"path"`
	Label     string   `json:"label"`
	Type      string   `json:"type,omitempty"`
	MappedBy  []string `json:"mapped_by"`
	Satisfied bool     `json:"satisfied"`
}

type ConnectionMappingWriteReadiness struct {
	Create              bool     `json:"create"`
	Upsert              bool     `json:"upsert"`
	MissingCreateFields []string `json:"missing_create_fields"`
}

type ConnectionMappingModel struct {
	ID                         string                                       `json:"id"`
	DeveloperIRGeneration      int                                          `json:"developer_ir_generation"`
	DeveloperIRHash            string                                       `json:"developer_ir_hash"`
	Fields                     []ConnectionMappingField                     `json:"fields"`
	ProviderFields             []map[string]any                             `json:"provider_fields"`
	ProviderCreateRequirements []ConnectionMappingProviderCreateRequirement `json:"provider_create_requirements"`
	WriteReadiness             ConnectionMappingWriteReadiness              `json:"write_readiness"`
	SchemaObservedAt           string                                       `json:"schema_observed_at,omitempty"`
	SchemaReady                bool                                         `json:"schema_ready"`
}

type ConnectionMappingSchema struct {
	LinkedAccount map[string]any           `json:"linked_account"`
	OAuthApp      map[string]any           `json:"oauth_app"`
	Status        string                   `json:"status"`
	Required      bool                     `json:"required"`
	Models        []ConnectionMappingModel `json:"models"`
	GeneratedAt   string                   `json:"generated_at"`
}

type ConnectionMappingJob struct {
	ID              string         `json:"id"`
	Action          string         `json:"action"`
	State           string         `json:"state"`
	LinkedAccountID string         `json:"linked_account_id"`
	Models          any            `json:"models,omitempty"`
	Result          map[string]any `json:"result,omitempty"`
	ErrorType       string         `json:"error_type,omitempty"`
	Error           string         `json:"error,omitempty"`
	CreatedAt       *float64       `json:"created_at,omitempty"`
	UpdatedAt       *float64       `json:"updated_at,omitempty"`
	FinishedAt      *float64       `json:"finished_at,omitempty"`
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
	WorkspaceID     string `json:"wsid"`
	LinkedAccountID string `json:"linked_account_id"`
	// Provider is retained for source compatibility with earlier SDKs. The V1
	// record API does not currently emit it, so it is optional on the wire.
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model"`
	UnifiedID      string         `json:"unified_id"`
	Data           map[string]any `json:"data"`
	RemoteData     any            `json:"remote_data,omitempty"`
	DeletedAt      *float64       `json:"deleted_at,omitempty"`
	DeletionSource string         `json:"deletion_source,omitempty"`
	UpdatedAt      *float64       `json:"updated_at,omitempty"`
}

type UnifiedRecordPage struct {
	Records    []UnifiedRecord `json:"records"`
	NextCursor *string         `json:"next_cursor"`
}

type ListRecordsOptions struct {
	Cursor            string
	PageSize          int
	IncludeRemoteData bool
	IncludeDeleted    bool
}

type RecordActionScope struct {
	WorkspaceID     string
	LinkedAccountID string
	Model           string
	IdempotencyKey  string
}

type CreateRecordParams struct {
	RecordActionScope
	Changes map[string]any
}

type UpsertRecordParams struct {
	RecordActionScope
	UnifiedID string
	Changes   map[string]any
}

type DeleteRecordParams struct {
	RecordActionScope
	UnifiedID string
}

type BulkRecordOperation string

const (
	BulkRecordCreate BulkRecordOperation = "create"
	BulkRecordUpdate BulkRecordOperation = "update"
	BulkRecordUpsert BulkRecordOperation = "upsert"
	BulkRecordDelete BulkRecordOperation = "delete"
)

type BulkRecordActionItem struct {
	Operation BulkRecordOperation `json:"operation"`
	UnifiedID string              `json:"unified_id,omitempty"`
	RemoteID  string              `json:"remote_id,omitempty"`
	Changes   map[string]any      `json:"changes"`
}

type BulkRecordActionParams struct {
	RecordActionScope
	Items []BulkRecordActionItem
}

type CreateCustomFieldParams struct {
	RecordActionScope
	Definition map[string]any
}

type Writeback struct {
	ID              string   `json:"id"`
	WorkflowID      string   `json:"workflow_id"`
	WorkspaceID     string   `json:"wsid,omitempty"`
	LinkedAccountID string   `json:"linked_account_id,omitempty"`
	Provider        string   `json:"provider,omitempty"`
	Model           string   `json:"model,omitempty"`
	UnifiedID       string   `json:"unified_id,omitempty"`
	State           string   `json:"state"`
	Result          any      `json:"result,omitempty"`
	ErrorType       string   `json:"error_type,omitempty"`
	CreatedAt       *float64 `json:"created_at,omitempty"`
	UpdatedAt       *float64 `json:"updated_at,omitempty"`
}

type WaitForWritebackOptions struct {
	Interval       time.Duration
	Timeout        time.Duration
	TerminalStates []string
}

type RecordWebhookEvent string

const (
	RecordWebhookEventCreated RecordWebhookEvent = "record.created"
	RecordWebhookEventUpdated RecordWebhookEvent = "record.updated"
	RecordWebhookEventDeleted RecordWebhookEvent = "record.deleted"
)

// DomainWebhookEvent deliberately excludes the high-frequency record.* CDC
// family, which has its own RecordWebhookEvent contract.
type DomainWebhookEvent string

const (
	DomainWebhookEventLinkedAccountCreated        DomainWebhookEvent = "linked_account.created"
	DomainWebhookEventLinkedAccountReconnected    DomainWebhookEvent = "linked_account.reconnected"
	DomainWebhookEventSyncRunning                 DomainWebhookEvent = "sync.running"
	DomainWebhookEventSyncCompleted               DomainWebhookEvent = "sync.completed"
	DomainWebhookEventSyncFailed                  DomainWebhookEvent = "sync.failed"
	DomainWebhookEventSyncDeletionsInferred       DomainWebhookEvent = "sync.deletions_inferred"
	DomainWebhookEventWritebackState              DomainWebhookEvent = "writeback.state"
	DomainWebhookEventProviderSchemaDriftDetected DomainWebhookEvent = "provider.schema.drift_detected"
	DomainWebhookEventProviderSchemaDriftResolved DomainWebhookEvent = "provider.schema.drift_resolved"
	DomainWebhookEventChangesetApproved           DomainWebhookEvent = "changeset.approved"
	DomainWebhookEventChangesetMerged             DomainWebhookEvent = "changeset.merged"
	DomainWebhookEventModelMigrationDispatched    DomainWebhookEvent = "model_migration.dispatched"
	DomainWebhookEventAgentJobCompleted           DomainWebhookEvent = "agent.job.completed"
	DomainWebhookEventWorkspaceInviteCreated      DomainWebhookEvent = "workspace.invite.created"
	DomainWebhookEventWorkspaceMemberJoined       DomainWebhookEvent = "workspace.member.joined"
	DomainWebhookEventWebhookEndpointCreated      DomainWebhookEvent = "webhook.endpoint.created"
	DomainWebhookEventWebhookEndpointUpdated      DomainWebhookEvent = "webhook.endpoint.updated"
	DomainWebhookEventWebhookEndpointDisabled     DomainWebhookEvent = "webhook.endpoint.disabled"
)

// WebhookEnvelope is the legacy record-event envelope. New integrations should
// unmarshal record CDC events into RecordWebhookEnvelope and control-plane
// events into DomainWebhookEnvelope. It remains source-compatible for callers
// that inspect record data dynamically.
type WebhookEnvelope struct {
	Event       string                   `json:"event"`
	Ctx         WebhookContext           `json:"ctx"`
	Data        map[string]any           `json:"data"`
	Breadcrumbs RecordWebhookBreadcrumbs `json:"breadcrumbs"`
}

// RecordWebhookEnvelope is emitted by the record CDC delivery path. Verify the
// raw request bytes with VerifyWebhookSignature before unmarshalling it.
type RecordWebhookEnvelope struct {
	Event       RecordWebhookEvent       `json:"event"`
	Ctx         RecordWebhookContext     `json:"ctx"`
	Data        RecordWebhookData        `json:"data"`
	Breadcrumbs RecordWebhookBreadcrumbs `json:"breadcrumbs"`
}

// DomainWebhookEnvelope is emitted by the durable control-plane outbox for
// sync, connection, mapping, and lifecycle events.
type DomainWebhookEnvelope struct {
	Event       DomainWebhookEvent       `json:"event"`
	Ctx         DomainWebhookContext     `json:"ctx"`
	Data        map[string]any           `json:"data"`
	Breadcrumbs DomainWebhookBreadcrumbs `json:"breadcrumbs"`
}

type WebhookDeliveryUser struct {
	OriginID *string `json:"origin_id"`
	ActorID  *string `json:"actor_id"`
}

type WebhookDeliveryIntegration struct {
	Provider   *string  `json:"provider"`
	Name       *string  `json:"name"`
	Categories []string `json:"categories"`
}

type RecordWebhookContext struct {
	Workspace   RecordWebhookWorkspace     `json:"workspace"`
	User        WebhookDeliveryUser        `json:"user"`
	Integration WebhookDeliveryIntegration `json:"integration"`
	OAuthApp    RecordWebhookOAuthApp      `json:"oauth_app"`
	Connection  RecordWebhookConnection    `json:"connection"`
}

type RecordWebhookWorkspace struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type RecordWebhookOAuthApp struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

type RecordWebhookConnection struct {
	ID string `json:"id"`
}

type DomainWebhookContext struct {
	Workspace   DomainWebhookWorkspace     `json:"workspace"`
	User        WebhookDeliveryUser        `json:"user"`
	Integration WebhookDeliveryIntegration `json:"integration"`
	OAuthApp    DomainWebhookOAuthApp      `json:"oauth_app"`
	Connection  DomainWebhookConnection    `json:"connection"`
}

type DomainWebhookWorkspace struct {
	ID string `json:"id"`
}

type DomainWebhookOAuthApp struct {
	ID *string `json:"id"`
}

type DomainWebhookConnection struct {
	ID *string `json:"id"`
}

type WebhookContext struct {
	Workspace   WebhookWorkspace   `json:"workspace"`
	User        WebhookUser        `json:"user"`
	Integration WebhookIntegration `json:"integration"`
	OAuthApp    WebhookOAuthApp    `json:"oauth_app"`
	Connection  WebhookConnection  `json:"connection"`
}

type WebhookWorkspace struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
}

type WebhookUser struct {
	OriginID *string `json:"origin_id,omitempty"`
	ActorID  *string `json:"actor_id,omitempty"`
}

type WebhookIntegration struct {
	Provider   *string  `json:"provider,omitempty"`
	Name       *string  `json:"name,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type WebhookOAuthApp struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

type WebhookConnection struct {
	ID *string `json:"id"`
}

type RecordWebhookData struct {
	Model     string         `json:"model"`
	RemoteID  string         `json:"remote_id"`
	UnifiedID string         `json:"unified_id"`
	Record    map[string]any `json:"record"`
	DeletedAt any            `json:"deleted_at"`
}

type RecordWebhookBreadcrumbs struct {
	EventID         string      `json:"event_id"`
	OccurredAt      string      `json:"occurred_at"`
	ContentHash     string      `json:"content_hash"`
	SyncRunID       *string     `json:"sync_run_id"`
	SourceCursor    any         `json:"source_cursor"`
	SourceUpdatedAt any         `json:"source_updated_at"`
	Plan            WebhookPlan `json:"plan"`
}

// Deprecated aliases preserve source compatibility with the original record
// webhook type names.
type WebhookData = RecordWebhookData
type WebhookBreadcrumbs = RecordWebhookBreadcrumbs

type DomainWebhookBreadcrumbs struct {
	EventID    string              `json:"event_id"`
	OccurredAt *string             `json:"occurred_at"`
	Entity     DomainWebhookEntity `json:"entity"`
	RequestID  *string             `json:"request_id"`
}

type DomainWebhookEntity struct {
	Type *string `json:"type"`
	ID   *string `json:"id"`
}

type WebhookPlan struct {
	Generation *int    `json:"generation"`
	Hash       *string `json:"hash"`
}
