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

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (*Repository, error) {
	repo := &Repository{
		collection: db.Collection(cliniciansCollectionName),
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

type Repository struct {
	collection *mongo.Collection
}

func (r *Repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "userId", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueClinicianUserId").
				SetPartialFilterExpression(bson.D{{"userId", bson.M{"$exists": true}}}),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "inviteId", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueInviteId").
				SetPartialFilterExpression(bson.D{{"inviteId", bson.M{"$exists": true}}}),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "email", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueClinicMemberEmail").
				SetPartialFilterExpression(bson.D{{"email", bson.M{"$exists": true}}}),
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

func (r *Repository) Get(ctx context.Context, clinicId string, clinicianId string) (*Clinician, error) {
	selector := clinicianSelector(clinicId, clinicianId)
	return r.getOne(ctx, selector)
}

func (r *Repository) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error) {
	opts := options.Find().
		SetLimit(int64(pagination.Limit)).
		SetSkip(int64(pagination.Offset))

	selector := bson.M{}
	if filter.ClinicId != nil {
		clinicObjId, _ := primitive.ObjectIDFromHex(*filter.ClinicId)
		selector["clinicId"] = clinicObjId
	}
	if filter.Email != nil {
		selector["email"] = filter.Email
	}
	if filter.UserId != nil {
		selector["userId"] = filter.UserId
	}
	if filter.Search != nil {
		selector["$text"] = bson.M{
			"$search": filter.Search,
		}
		textScore := bson.M{
			"score": bson.M{
				"$meta": "textScore",
			},
		}
		opts.SetProjection(textScore)
		opts.SetSort(textScore)
	}
	cursor, err := r.collection.Find(ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing clinicians: %w", err)
	}

	var clinicians []*Clinician
	if err = cursor.All(ctx, &clinicians); err != nil {
		return nil, fmt.Errorf("error decoding clinicians list: %w", err)
	}

	return clinicians, nil
}

func (r *Repository) Create(ctx context.Context, clinician *Clinician) (*Clinician, error) {
	if exists, err := r.clinicianExists(ctx, clinician); err != nil {
		return nil, fmt.Errorf("error checking for duplicate clinicians: %v", err)
	} else if exists {
		return nil, ErrDuplicate
	}

	res, err := r.collection.InsertOne(ctx, clinician)
	if err != nil {
		return nil, fmt.Errorf("error creating clinician: %w", err)
	}

	selector := bson.M{
		"_id": res.InsertedID.(primitive.ObjectID),
	}
	return r.getOne(ctx, selector)
}

func (r *Repository) Update(ctx context.Context, clinicId string, id string, clinician *Clinician) (*Clinician, error) {
	removeUnmodifiableFields(clinician)
	selector := clinicianSelector(clinicId, id)
	update := bson.M{
		"$set": clinician,
	}

	return r.updateOne(ctx, selector, update)
}

func (r *Repository) Delete(ctx context.Context, clinicId string, userId string) error {
	return r.deleteOne(ctx, clinicianSelector(clinicId, userId))
}

func (r *Repository) GetInvite(ctx context.Context, clinicId, inviteId string) (*Clinician, error) {
	return r.getOne(ctx, inviteSelector(clinicId, inviteId))
}

func (r *Repository) DeleteInvite(ctx context.Context, clinicId, inviteId string) error {
	return r.deleteOne(ctx, inviteSelector(clinicId, inviteId))
}

func (r *Repository) AssociateInvite(ctx context.Context, associate AssociateInvite) (*Clinician, error) {
	if associate.InviteId == "" {
		return nil, fmt.Errorf("inviteId cannot be empty")
	}
	if associate.UserId == "" {
		return nil, fmt.Errorf("userId cannot be empty")
	}
	selector := inviteSelector(associate.ClinicId, associate.InviteId)
	invite, err := r.getOne(ctx, selector)
	if err != nil {
		return nil, err
	}

	idSelector := bson.M{
		"_id": invite.Id,
	}
	set := bson.M{
		"userId": associate.UserId,
	}
	unset := bson.M{
		"inviteId": associate.InviteId,
	}

	if associate.ClinicianName != nil {
		set["fullName"] = associate.ClinicianName
	}

	update := bson.M{
		"$set": set,
		"$unset": unset,
	}
	return r.updateOne(ctx, idSelector, update)
}

func (r *Repository) getOne(ctx context.Context, selector bson.M) (*Clinician, error) {
	clinician := &Clinician{}
	err := r.collection.FindOne(ctx, selector).Decode(&clinician)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinician, nil
}

func (r *Repository) updateOne(ctx context.Context, selector, update bson.M) (*Clinician, error) {
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to update clinician: %w", err)
	}

	return r.getOne(ctx, selector)
}

func (r *Repository) deleteOne(ctx context.Context, selector bson.M) error {
	res, err := r.collection.DeleteOne(ctx, selector)
	if err != nil {
		return fmt.Errorf("unable to delete clincian: %w", err)
	}
	if res.DeletedCount == int64(0) {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) clinicianExists(ctx context.Context, clinician *Clinician) (bool, error) {
	or := make([]bson.M, 0)
	if clinician.ClinicId != nil {
		if clinician.UserId != nil {
			or = append(or, bson.M{
				"clinicId": clinician.ClinicId,
				"userId":   clinician.UserId,
			})
		}
		if clinician.InviteId != nil {
			or = append(or, bson.M{
				"clinicId": clinician.ClinicId,
				"inviteId": clinician.InviteId,
			})
		}
		if clinician.Email != nil {
			or = append(or, bson.M{
				"clinicId": clinician.ClinicId,
				"email":    clinician.Email,
			})
		}
	}

	if len(or) == 0 {
		return false, errors.New("invalid clinician selector")
	}

	count, err := r.collection.CountDocuments(ctx, bson.M{
		"$or": or,
	})
	return count > int64(0), err
}

func clinicianSelector(clinicId, clinicianId string) bson.M {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	return bson.M{
		"userId":   clinicianId,
		"clinicId": clinicObjId,
	}
}

func inviteSelector(clinicId, inviteId string) bson.M {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	return bson.M{
		"inviteId": inviteId,
		"clinicId": clinicObjId,
	}
}

func removeUnmodifiableFields(clinician *Clinician) {
	clinician.ClinicId = nil
}
