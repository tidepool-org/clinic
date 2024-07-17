package clinicians

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
)

const (
	CollectionName = "clinicians"
)

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (*Repository, error) {
	repo := &Repository{
		collection: db.Collection(CollectionName),
		logger:     logger,
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
	logger     *zap.SugaredLogger
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
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "role", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("ClinicianByRole"),
		},
		{
			Keys: bson.D{
				{Key: "createdTime", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("CliniciansByCreatedTime"),
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
		SetSort(bson.D{{"createdTime", -1}}).
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
	if filter.Role != nil {
		selector["roles"] = bson.M{
			"$elemMatch": bson.M{
				"$eq": filter.Role,
			},
		}
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

	clinician.CreatedTime = time.Now()
	clinician.UpdatedTime = time.Now()
	res, err := r.collection.InsertOne(ctx, clinician)
	if err != nil {
		return nil, fmt.Errorf("error creating clinician: %w", err)
	}

	selector := bson.M{
		"_id": res.InsertedID.(primitive.ObjectID),
	}
	return r.getOne(ctx, selector)
}

func (r *Repository) Update(ctx context.Context, update *ClinicianUpdate) (*Clinician, error) {
	selector := clinicianSelector(update.ClinicId, update.ClinicianId)
	clinician, err := r.getOne(ctx, selector)
	if err != nil {
		return nil, err
	}

	updates := bson.M{
		"$set": bson.M{
			"roles":       update.Clinician.Roles,
			"name":        update.Clinician.Name,
			"updatedTime": time.Now(),
		},
	}

	if clinician.RolesChanged(update.Clinician.Roles) {
		// Used for optimistic locking
		selector["updatedTime"] = clinician.UpdatedTime

		// Keep track of the user who updated clinician's roles
		updates["$push"] = bson.M{
			"rolesUpdates": RolesUpdate{
				Roles:     update.Clinician.Roles,
				UpdatedBy: update.UpdatedBy,
			},
		}
	}

	return r.updateOne(ctx, selector, updates)
}

func (r *Repository) UpdateAll(ctx context.Context, update *CliniciansUpdate) error {
	selector := bson.M{
		"userId": update.UserId,
	}
	updates := bson.M{
		"$set": bson.M{
			"email":       update.Email,
			"updatedTime": time.Now(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, selector, updates)
	if result != nil && result.MatchedCount > 0 && result.MatchedCount > result.ModifiedCount {
		if store.IsDuplicateKeyError(err) {
			r.logger.Warnw("unable to update all records", "userId", update.UserId, "error", err)
			err = fmt.Errorf("%w: duplicate email", errors.ConstraintViolation)
		}
		err = fmt.Errorf("partially updated %v out of %v clinician records: %w", result.ModifiedCount, result.MatchedCount, err)
	}

	return err
}

func (r *Repository) Delete(ctx context.Context, clinicId string, userId string) error {
	return r.deleteOne(ctx, clinicianSelector(clinicId, userId))
}

func (r *Repository) DeleteAll(ctx context.Context, clinicId string) error {
	_, err := r.collection.DeleteMany(ctx, allCliniciansSelector(clinicId))
	return err
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
		"userId":      associate.UserId,
		"updatedTime": time.Now(),
	}
	unset := bson.M{
		"inviteId": associate.InviteId,
	}

	if associate.ClinicianName != nil {
		set["name"] = associate.ClinicianName
	}

	update := bson.M{
		"$set":   set,
		"$unset": unset,
	}
	return r.updateOne(ctx, idSelector, update)
}

func (r *Repository) getOne(ctx context.Context, selector bson.M) (*Clinician, error) {
	clinician := &Clinician{}
	err := r.collection.FindOne(ctx, selector).Decode(clinician)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinician, nil
}

func (r *Repository) updateOne(ctx context.Context, selector, update bson.M) (*Clinician, error) {
	result := r.collection.FindOneAndUpdate(ctx, selector, update)
	err := result.Err()

	if result.Err() == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to update clinician: %w", err)
	}

	beforeUpdate := Clinician{}
	if err := result.Decode(&beforeUpdate); err != nil {
		return nil, err
	}

	return r.getOne(ctx, bson.M{"_id": beforeUpdate.Id})
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
		return false, fmt.Errorf("invalid clinician selector")
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

func allCliniciansSelector(clinicId string) bson.M {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	return bson.M{
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
