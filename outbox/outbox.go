package outbox

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const CollectionName = "outbox"

// EventType identifies the kind of event
type EventType string

const (
	EventTypeSendProviderConnectionEmail EventType = "sendProviderConnectionEmail"
)

// Event is the common envelope for all outbox events
type Event struct {
	Id          *primitive.ObjectID `bson:"_id,omitempty"`
	EventType   EventType           `bson:"eventType"`
	CreatedTime time.Time           `bson:"createdTime"`
	Payload     bson.Raw            `bson:"payload"`
}

// SendProviderConnectionEmailPayload is the payload for sendProviderConnectionEmail events
type SendProviderConnectionEmailPayload struct {
	ClinicId     string `bson:"clinicId"`
	ClinicName   string `bson:"clinicName"`
	PatientEmail string `bson:"patientEmail"`
	PatientName  string `bson:"patientName"`
	ProviderName string `bson:"providerName"`
}

//go:generate go tool mockgen -source=./outbox.go -destination=./test/mock_outbox.go -package test

type Repository interface {
	Create(ctx context.Context, event Event) error
	Initialize(ctx context.Context) error
}

// NewEvent creates an Event from a typed payload
func NewEvent(eventType EventType, payload interface{}) (Event, error) {
	raw, err := bson.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("error marshaling outbox event payload: %w", err)
	}

	return Event{
		EventType:   eventType,
		CreatedTime: time.Now(),
		Payload:     bson.Raw(raw),
	}, nil
}
