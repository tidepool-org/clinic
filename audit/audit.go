package audit

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ExportPatientListEvent = "export_patient_list"
)

type AuditEvent struct {
	Id          *primitive.ObjectID `bson:"_id,omitempty"`
	ClinicianID string              `bson:"clinicianID,omitempty"`
	ClinicID    string              `bson:"clinicID,omitempty"`
	CreatedTime time.Time           `bson:"createdTime,omitempty"`
	EventName   string              `bson:"eventName,omitempty"`
}

type AuditEventRecorder interface {
	Create(ctx context.Context, event AuditEvent) error
}
