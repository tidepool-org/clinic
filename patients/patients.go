package patients

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)


var (
	ErrNotFound         = fmt.Errorf("patient %w", errors.NotFound)
	ErrDuplicatePatient = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)
	ErrDuplicateEmail   = fmt.Errorf("%w: email address is already taken", errors.Duplicate)

	permission = make(Permission, 0)
	CustodialAccountPermissions = Permissions{
		Custodian: &permission,
		View:      &permission,
		Upload:    &permission,
		Note:      &permission,
	}
)

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Patient, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, clinicId string, userId string, patient Patient) (*Patient, error)
	Remove(ctx context.Context, clinicId string, userId string) error
	UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error)
	DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error)
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
	Permissions   *Permissions        `bson:"permissions,omitempty"`
}

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type Filter struct {
	ClinicId *string
	UserId   *string
	Search   *string
}

type Permission = map[string]interface{}
type Permissions struct {
	Custodian *Permission `bson:"custodian,omitempty"`
	View      *Permission `bson:"view,omitempty"`
	Upload    *Permission `bson:"upload,omitempty"`
	Note      *Permission `bson:"note,omitempty"`
}

func (p *Permissions) Empty() bool {
	return p.Custodian == nil &&
		p.View == nil &&
		p.Upload == nil &&
		p.Note == nil
}
