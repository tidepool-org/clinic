package patients

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	List(ctx context.Context, clinicId string, filter *Filter, pagination store.Pagination) ([]*Patient, error)
	Create(ctx context.Context, clinicId string, patient Patient) (*Patient, error)
	Update(ctx context.Context, clinicId string, patient Patient) (*Patient, error)
	Delete(ctx context.Context, clinicId string, userId string) error
}

type Patient struct {
	Id            primitive.ObjectID  `bson:"_id,omitempty"`
	UserId        *string             `bson:"userId,omitempty"`
	ClinicId      primitive.ObjectID  `bson:"clinicId,omitempty"`
	BirthDate     *string             `bson:"birthDate,omitempty"`
	Email         *string             `bson:"email,omitempty"`
	FullName      *string             `bson:"fullName,omitempty"`
	Mrn           *string             `bson:"mrn,omitempty"`
	TargetDevices []string            `bson:"targetDevices,omitempty"`
	Permissions   *PatientPermissions `bson:"permissions,omitempty"`
}

type Filter struct {
	Search *string
}

type PatientPermissions struct {
	Upload *map[string]interface{} `bson:"upload,omitempty"`
	View   *map[string]interface{} `bson:"view,omitempty"`
	Note   *map[string]interface{} `bson:"note,omitempty"`
	Root   *map[string]interface{} `bson:"root,omitempty"`
}
