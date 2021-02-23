package clinicians

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
)

const (
	cliniciansCollectionName = "clinicians"
)

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (Service, error) {
	repo := &repository{
		collection: db.Collection(cliniciansCollectionName),
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

type repository struct {
	collection *mongo.Collection
}

//type Clinician struct {
//	Id          primitive.ObjectID `bson:"_id,omitempty"`
//	Email       *string            `bson:"email,omitempty"`
//	UserId      *string            `bson:"userId,omitempty"`
//	InviteId    *string            `bson:"inviteId,omitempty"`
//	Name        *string            `bson:"name,omitempty"`
//	Permissions []string           `bson:"permissions,omitempty"`
//}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "userId", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueClinicianUserId"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "inviteId", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueInviteId"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "email", Value: "text"},
				{Key: "fullName", Value: "text"},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("ClinicianSearch"),
		},
	})
	return err
}

func (r *repository) Get(ctx context.Context, clinicId string, userId string) (*Clinician, error) {
	selector := clinicianSelector(clinicId, userId)
	clinician := &Clinician{}
	err := r.collection.FindOne(ctx, selector).Decode(&clinician)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinician, nil
}

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(filter.ClinicId)
	opts := options.Find().
		SetLimit(int64(pagination.Limit)).
		SetSkip(int64(pagination.Offset))

	selector := bson.M{
		"clinicId": clinicObjId,
	}
	if filter.Search != nil {
		selector["$text"] = bson.M{
			"$search": filter.Search,
		}
		opts.SetSort(bson.M{
			"score": bson.M{
				"$meta": "textScore",
			},
		})
	}
	cursor, err := r.collection.Find(ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing clinicians: %w", err)
	}

	var clinicians []*Clinician
	if err = cursor.All(ctx, clinicians); err != nil {
		return nil, fmt.Errorf("error decoding clinicians list: %w", err)
	}

	return clinicians, nil
}

func (r *repository) Create(ctx context.Context, clinician *Clinician) (*Clinician, error) {
	if exists, err := r.clinicianExists(ctx, clinician); err != nil {
		return nil, fmt.Errorf("error checking for duplicate clinicians: %v", err)
	} else if exists {
		return nil, ErrDuplicate
	}

	res, err := r.collection.InsertOne(ctx, clinician)
	if err != nil {
		return nil, fmt.Errorf("error creating clinician: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID)
	return r.Get(ctx, clinician.ClinicId.Hex(), id.Hex())
}

func (r *repository) Update(ctx context.Context, clinicId string, userId string, clinician *Clinician) (*Clinician, error) {
	removeUnmodifiableFieldsFromClinicMember(clinician)
	selector := clinicianSelector(clinicId, userId)
	update := bson.M{
		"$set": clinician,
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		return nil, fmt.Errorf("unable to update clinician: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}

func (r *repository) Delete(ctx context.Context, clinicId string, userId string) error {
	selector := clinicianSelector(clinicId, userId)
	res, err := r.collection.DeleteOne(ctx, selector)
	if err != nil {
		return fmt.Errorf("unable to delete clincian: %w", err)
	}
	if res.DeletedCount == int64(0) {
		return ErrNotFound
	}

	return nil
}

func (r *repository) GetByInviteId(ctx context.Context, clinicId string, inviteId string) (*Clinician, error) {
	selector := invitedSelector(clinicId, inviteId)
	clinician := &Clinician{}
	err := r.collection.FindOne(ctx, selector).Decode(clinician)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinician, nil
}

func (r *repository) UpdateByInviteId(ctx context.Context, clinicId string, inviteId string, clinician *Clinician) (*Clinician, error) {
	removeUnmodifiableFieldsFromClinicInvitee(clinician)
	selector := invitedSelector(clinicId, inviteId)
	update := bson.M{
		"$set": clinician,
	}
	if clinician.InviteId == nil && clinician.UserId != nil {
		update["$unset"] = bson.M{
			"inviteId": 1,
		}
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		return nil, fmt.Errorf("unable to update clinician: %w", err)
	}
	if clinician.InviteId == nil && clinician.UserId != nil {
		return r.Get(ctx, clinicId, *clinician.UserId)
	}

	return r.Get(ctx, clinicId, inviteId)
}

func (r *repository) DeleteByInviteId(ctx context.Context, clinicId string, inviteId string) error {
	selector := clinicianSelector(clinicId, inviteId)
	res, err := r.collection.DeleteOne(ctx, selector)
	if err != nil {
		return fmt.Errorf("unable to delete clincian: %w", err)
	}
	if res.DeletedCount == int64(0) {
		return ErrNotFound
	}

	return nil
}

func (r *repository) clinicianExists(ctx context.Context, clinician *Clinician) (bool, error) {
	var or []bson.M
	selector := bson.M{
		"$or": or,
	}
	if clinician.ClinicId != nil && clinician.UserId != nil {
		or = append(or, bson.M{
			"clinicId": clinician.ClinicId,
			"userId": clinician.UserId,
		})
	}
	if clinician.ClinicId != nil && clinician.InviteId != nil {
		or = append(or, bson.M{
			"clinicId": clinician.ClinicId,
			"inviteId": clinician.InviteId,
		})
	}
	if len(or) == 0 {
		return false, errors.New("invalid clinician selector")
	}

	count, err := r.collection.CountDocuments(ctx, selector)
	return count > int64(0), err
}

func clinicianSelector(clinicId string, userId string) bson.M {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	return bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
	}
}

func invitedSelector(clinicId string, inviteId string) bson.M {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	return bson.M{
		"clinicId": clinicObjId,
		"inviteId": inviteId,
	}
}

func removeUnmodifiableFieldsFromClinicMember(clinician *Clinician) {
	clinician.InviteId = nil
	clinician.ClinicId = nil
	clinician.UserId = nil
}

func removeUnmodifiableFieldsFromClinicInvitee(clinician *Clinician) {
	clinician.ClinicId = nil
}