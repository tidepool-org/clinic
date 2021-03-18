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
	clinics, err := c.List(ctx, &Filter{Email: clinic.Email}, store.Pagination{Limit: 1, Offset: 0})
	if err != nil {
		return nil, fmt.Errorf("error finding clinic by Email address: %w", err)
	}
	// Fail gracefully if there is a duplicate Email address
	if len(clinics) > 0 {
		return nil, ErrDuplicateEmail
	}

	// Insertion will fail if there are two concurrent requests, which are both
	// trying to create a clinic with the same Email address
	res, err := c.collection.InsertOne(ctx, clinic)
	if err != nil {
		return nil, fmt.Errorf("error creating clinic: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID)
	return c.Get(ctx, id.Hex())
}

func (c *repository) Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error) {
	if clinic.Email != nil {
		if c, err := c.FindByEmail(ctx, *clinic.Email); err != nil {
			return nil, err
		} else if c != nil {
			return nil, ErrDuplicateEmail
		}
	}
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}
	err := c.collection.FindOneAndUpdate(ctx, selector, createUpdateDocument(clinic)).Err()
	if err != nil {
		return nil, fmt.Errorf("error updating clinic: %w", err)
	}

	return c.Get(ctx, id)
}

func (c *repository) FindByEmail(ctx context.Context, email string) (*Clinic, error) {
	clinics, err := c.List(ctx, &Filter{Email: &email}, store.Pagination{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("error finding clinic by email address: %w", err)
	}

	if len(clinics) == 0 {
		return nil, nil
	}

	return clinics[0], nil
}

func createUpdateDocument(clinic *Clinic) bson.M {
	update := bson.M{}
	if clinic != nil {
		// Make sure we're not overriding the id
		clinic.Id = nil
		update["$set"] = clinic
	}

	return update
}