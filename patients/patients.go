package patients

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = fmt.Errorf("patient %w", errors.NotFound)
var ErrDuplicate = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Patient, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, clinicId string, userId string, patient Patient) (*Patient, error)
}

type Patient struct {
	Id            *primitive.ObjectID `bson:"_id,omitempty"`
	ClinicId      *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId        *string             `bson:"userId,omitempty"`
	BirthDate     *string             `bson:"birthDate"`
	Email         *string             `bson:"email"`
	FullName      *string             `bson:"fullName"`
	Mrn           *string             `bson:"mrn"`
	TargetDevices *[]string           `bson:"targetDevices"`
	Permissions   *PatientPermissions `bson:"permissions,omitempty"`
}

type Filter struct {
	ClinicId string
	UserId   *string
	Search   *string
}

type PatientPermissions struct {
	Custodian *map[string]interface{} `bson:"custodian,omitempty"`
	Note      *map[string]interface{} `bson:"note,omitempty"`
	View      *map[string]interface{} `bson:"view,omitempty"`
	Root      *map[string]interface{} `bson:"root,omitempty"`
	Upload    *map[string]interface{} `bson:"upload,omitempty"`
}
