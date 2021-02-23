package patients

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = errors.New("patient not found")
var ErrDuplicate = errors.New("patient is already a member of the clinic")

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Patient, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, patient Patient) (*Patient, error)
}

type Patient struct {
	Id            *primitive.ObjectID `bson:"_id,omitempty"`
	ClinicId      *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId        *string             `bson:"userId,omitempty"`
	BirthDate     *string             `bson:"birthDate,omitempty"`
	Email         *string             `bson:"email,omitempty"`
	FullName      *string             `bson:"fullName,omitempty"`
	Mrn           *string             `bson:"mrn,omitempty"`
	TargetDevices *[]string           `bson:"targetDevices,omitempty"`
	Permissions   *PatientPermissions `bson:"permissions,omitempty"`
}

type Filter struct {
	ClinicId string
	UserId   *string
	Search   *string
}

type PatientPermissions struct {
	Upload *map[string]interface{} `bson:"upload,omitempty"`
	View   *map[string]interface{} `bson:"view,omitempty"`
	Note   *map[string]interface{} `bson:"note,omitempty"`
	Root   *map[string]interface{} `bson:"root,omitempty"`
}
