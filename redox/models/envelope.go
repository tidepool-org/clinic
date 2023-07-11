package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MessageEnvelope struct {
	Id primitive.ObjectID `bson:"_id"`

	// Copy of meta from the payload for easier deserialization
	Meta Meta

	// BSON encoded form of the original message
	Message bson.Raw
}

type Meta struct {
	DataModel    string `json:"DataModel"`
	Destinations *[]struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	} `json:"Destinations,omitempty"`
	EventDateTime *string `json:"EventDateTime"`
	EventType     string  `json:"EventType"`
	FacilityCode  *string `json:"FacilityCode"`
	Logs          *[]struct {
		AttemptID *string `json:"AttemptID"`
		ID        *string `json:"ID"`
	} `json:"Logs,omitempty"`
	Message *struct {
		ID *float32 `json:"ID"`
	} `json:"Message,omitempty"`
	Source *struct {
		ID   *string `json:"ID"`
		Name *string `json:"Name"`
	} `json:"Source,omitempty"`
	Test         *bool `json:"Test"`
	Transmission *struct {
		ID *float32 `json:"ID"`
	} `json:"Transmission,omitempty"`
}

func (m *Meta) IsValid() bool {
	return m.DataModel != "" && m.EventType != ""
}
