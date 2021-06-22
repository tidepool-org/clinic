package patients

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
	patientsCollectionName = "patients"
)

//go:generate mockgen --build_flags=--mod=mod -source=./repo.go -destination=./test/mock_repository.go -package test -aux_files=github.com/tidepool-org/clinic/patients=patients.go MockRepository

type Repository interface {
	Service
}

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (Repository, error) {
	repo := &repository{
		collection: db.Collection(patientsCollectionName),
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
				SetName("UniquePatient"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "email", Value: "text"},
				{Key: "fullName", Value: "text"},
				{Key: "firstName", Value: "text"},
				{Key: "lastName", Value: "text"},
				{Key: "mrn", Value: "text"},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSearch"),
		},
	})
	return err
}

func (r *repository) Get(ctx context.Context, clinicId string, userId string) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
	}

	patient := &Patient{}
	err := r.collection.FindOne(ctx, selector).Decode(&patient)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return patient, nil
}

func (r *repository) Remove(ctx context.Context, clinicId string, userId string) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
	}

	res, err := r.collection.DeleteOne(ctx, selector)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Patient, error) {
	opts := options.Find().
		SetLimit(int64(pagination.Limit)).
		SetSkip(int64(pagination.Offset))

	selector := bson.M{}
	if filter.ClinicId != nil {
		clinicId := *filter.ClinicId
		clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
		selector["clinicId"] = clinicObjId
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
		return nil, fmt.Errorf("error listing PatientsRepo: %w", err)
	}

	var patients []*Patient
	if err = cursor.All(ctx, &patients); err != nil {
		return nil, fmt.Errorf("error decoding PatientsRepo list: %w", err)
	}

	return patients, nil
}

func (r *repository) Create(ctx context.Context, patient Patient) (*Patient, error) {
	clinicId := patient.ClinicId.Hex()
	filter := &Filter{
		ClinicId: &clinicId,
		UserId: patient.UserId,
	}
	patients, err := r.List(ctx, filter, store.Pagination{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("error checking for duplicate PatientsRepo: %v", err)
	}
	if len(patients) > 0 {
		return nil, ErrDuplicatePatient
	}

	if _, err = r.collection.InsertOne(ctx, patient); err != nil {
		return nil, fmt.Errorf("error creating patient: %w", err)
	}

	return r.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
}

func (r *repository) Update(ctx context.Context, clinicId, userId string, patient Patient) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
	}

	update := bson.M{
		"$set": patient,
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}

func (r *repository) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
	}

	update := bson.M{}
	if permissions == nil {
		update["$unset"] = bson.M{
			"permissions": "",
		}
	} else {
		update["$set"] = bson.M{
			"permissions": permissions,
		}
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}

func (r *repository) DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error) {
	key := fmt.Sprintf("permissions.%s", permission)
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId": userId,
		key: bson.M{"$exists": true},
	}

	update := bson.M{
		"$unset": bson.D{{Key: key , Value: ""}},
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error removing permission: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}
