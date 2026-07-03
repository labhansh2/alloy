package notion

type WebhookPayload struct {
	Id             string         `json:"id"`               // UUID
	Timestamp      string         `json:"timestamp"`        // ISO 8601 formatted time
	WorkspaceId    string         `json:"workspace_id"`     // UUID
	SubscriptionId string         `json:"subscription_id"`  // UUID
	IntegrationId  string         `json:"integration_id"`   // UUID
	Type           EventType      `json:"type"`             // Type of event, e.g. page.created
	Authors        []Author       `json:"authors"`          // Array of author objects
	AccessibleBy   []AccessibleBy `json:"accessible_by"`    // Array of accessible bot/user objects: those who own the bot connection to the integration_id and have access to the entity
	AttemptNumber  int            `json:"attempt_number"`   // Attempt number (1-8) of delivery
	Entity         Entity         `json:"entity"`           // Object that triggered the event
	Data           map[string]any `json:"data"`             // Additional event-specific data
}

// Author represents who performed the action on the event, i.e. person, bot, or agent.
type Author struct {
	Id   string `json:"id"`   // ID of the author
	Type string `json:"type"` // "person", "bot", or "agent"
}

// AccessibleBy represents each accessible bot or user for public connections.
type AccessibleBy struct {
	Id   string `json:"id"`   // ID of the accessible entity
	Type string `json:"type"` // "person" or "bot"
}

// Entity describes the object that triggered the event.
type Entity struct {
	Id   string `json:"id"`   // ID of the entity
	Type string `json:"type"` // "page", "block", or "database"
}

type EventType string

const (
	// Page Events
	EventPageContentUpdated     EventType = "page.content_updated"
	EventPageCreated            EventType = "page.created"
	EventPageDeleted            EventType = "page.deleted"
	EventPageLocked             EventType = "page.locked"
	EventPageMoved              EventType = "page.moved"
	EventPagePropertiesUpdated  EventType = "page.properties_updated"
	EventPageUndeleted          EventType = "page.undeleted"
	EventPageUnlocked           EventType = "page.unlocked"

	// Database Events
	EventDatabaseContentUpdated    EventType = "database.content_updated"
	EventDatabaseCreated           EventType = "database.created"
	EventDatabaseDeleted           EventType = "database.deleted"
	EventDatabaseMoved             EventType = "database.moved"
	EventDatabaseSchemaUpdated     EventType = "database.schema_updated"
	EventDatabaseUndeleted         EventType = "database.undeleted"

	// Data Source Events (2025-09-03 API version)
	EventDataSourceContentUpdated  EventType = "data_source.content_updated"
	EventDataSourceCreated         EventType = "data_source.created"
	EventDataSourceDeleted         EventType = "data_source.deleted"
	EventDataSourceMoved           EventType = "data_source.moved"
	EventDataSourceSchemaUpdated   EventType = "data_source.schema_updated"
	EventDataSourceUndeleted       EventType = "data_source.undeleted"

	// Comment Events
	EventCommentCreated            EventType = "comment.created"
	EventCommentDeleted            EventType = "comment.deleted"
	EventCommentUpdated            EventType = "comment.updated"
)