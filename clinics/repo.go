package clinics

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"time"
)

const (
	clinicsCollectionName = "clinics"
)

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (Service, error) {
	repo := &repository{
		collection: db.Collection(clinicsCollectionName),
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

func (c *repository) Initialize(ctx context.Context) error {
	_, err := c.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "email", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueEmail"),
		},
		{
			Keys: bson.D{
				{Key: "shareCodes", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueShareCodes"),
		},
		{
			Keys: bson.D{
				{Key: "canonicalShareCode", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetUnique(true).
				SetName("UniqueCanonicalShareCode"),
		},
	})
	return err
}


func (c *repository) Get(ctx context.Context, id string) (*Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	clinic := &Clinic{}
	err := c.collection.FindOne(ctx, selector).Decode(&clinic)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinic, nil
}

func (c *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error) {
	opts := options.Find().
		SetSkip(int64(pagination.Offset)).
		SetLimit(int64(pagination.Limit))

	selector := bson.M{}
	if len(filter.Ids) > 0 {
		selector["_id"] = bson.M{"$in": store.ObjectIDSFromStringArray(filter.Ids)}
	}
	if filter.Email != nil {
		selector["email"] = filter.Email
	}
	if filter.ShareCodes != nil {
		selector["shareCodes"] = bson.M{
			"$in": filter.ShareCodes,
		}
	}
	cursor, err := c.collection.Find(ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing clinics: %w", err)
	}

	clinics := make([]*Clinic, 0)
	if err = cursor.All(ctx, &clinics); err != nil {
		return nil, fmt.Errorf("error decoding clinics list: %w", err)
	}

	return clinics, nil
}

func (c *repository) Create(ctx context.Context, clinic *Clinic) (*Clinic, error) {
	clinics, err := c.List(ctx, &Filter{ShareCodes: *clinic.ShareCodes}, store.Pagination{Limit: 1, Offset: 0})
	if err != nil {
		return nil, fmt.Errorf("error finding clinic by sharecode: %w", err)
	}
	// Fail gracefully if there is a clinic with duplicate share code
	if len(clinics) > 0 {
		return nil, ErrDuplicateShareCode
	}

	setCreatedAt(clinic)
	setUpdatedAt(clinic)

	// Insertion will fail if there are two concurrent requests, which are both
	// trying to create a clinic with the same share code
	res, err := c.collection.InsertOne(ctx, clinic)
	if err != nil {
		return nil, fmt.Errorf("error creating clinic: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID)
	return c.Get(ctx, id.Hex())
}

func (c *repository) Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	setUpdatedAt(clinic)
	update := createUpdateDocument(clinic)
	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		return nil, fmt.Errorf("error updating clinic: %w", err)
	}

	return c.Get(ctx, id)
}

func (c *repository) UpsertAdmin(ctx context.Context, id, clinicianId string) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}
	update := bson.M {
		"$addToSet": bson.M {
			"admins": clinicianId,
		},
		"$set": bson.M {
			"updatedAt": time.Now(),
		},
	}
	return c.collection.FindOneAndUpdate(ctx, selector, update).Err()
}

func (c *repository) RemoveAdmin(ctx context.Context, id, clinicianId string) error {
	clinic, err := c.Get(ctx, id)
	if err != nil {
		return err
	}

	selector := bson.M{
		"_id": clinic.Id,
		"updatedAt": clinic.UpdatedAt, // used for optimistic locking
	}
	update := bson.M {
		"$pull": bson.M {
			"admins": clinicianId,
		},
		"$set": bson.M {
			"updatedAt": time.Now(),
		},
	}

	var updated Clinic
	if err := c.collection.FindOneAndUpdate(ctx, selector, update).Decode(&updated); err != nil {
		return err
	}

	if updated.Admins == nil || len(*updated.Admins) == 0 {
		return ErrAdminRequired
	}

	return nil
}

func setUpdatedAt(clinic *Clinic) {
	clinic.UpdatedAt = time.Now()
}

func setCreatedAt(clinic *Clinic) {
	clinic.CreatedAt = time.Now()
}

func createUpdateDocument(clinic *Clinic) bson.M {
	update := bson.M{}
	if clinic != nil {
		// Make sure we're not overriding the id and sharecodes
		clinic.Id = nil
		clinic.ShareCodes = nil

		// Refresh updatedAt timestamp
		setUpdatedAt(clinic)

		update["$set"] = clinic
		if clinic.CanonicalShareCode != nil {
			update["$addToSet"] = bson.M{
				"shareCodes": *clinic.CanonicalShareCode,
			}
		}
	}

	return update
}
