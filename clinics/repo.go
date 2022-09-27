package clinics

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	createdTime := bson.M{}
	if filter.CreatedTimeStart != nil {
		createdTime["$gte"] = filter.CreatedTimeStart
	}
	if filter.CreatedTimeEnd != nil {
		createdTime["$lt"] = filter.CreatedTimeEnd
	}
	if len(createdTime) > 0 {
		selector["createdTime"] = createdTime
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

	setCreatedTime(clinic)
	setUpdatedTime(clinic)

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

	setUpdatedTime(clinic)
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
	update := bson.M{
		"$addToSet": bson.M{
			"admins": clinicianId,
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}
	return c.collection.FindOneAndUpdate(ctx, selector, update).Err()
}

func (c *repository) RemoveAdmin(ctx context.Context, id, clinicianId string, allowOrphaning bool) error {
	clinic, err := c.Get(ctx, id)
	if err != nil {
		return err
	}
	if !allowOrphaning && !canRemoveAdmin(*clinic, clinicianId) {
		return ErrAdminRequired
	}

	updatedTime := time.Now()
	selector := bson.M{
		"_id":         clinic.Id,
		"updatedTime": clinic.UpdatedTime, // used for optimistic locking
	}
	update := bson.M{
		"$pull": bson.M{
			"admins": clinicianId,
		},
		"$set": bson.M{
			"updatedTime": updatedTime,
		},
	}

	if res, err := c.collection.UpdateOne(ctx, selector, update); err != nil {
		return err
	} else if res.MatchedCount != int64(1) {
		return fmt.Errorf("concurrent modification of clinic detected")
	}

	return nil
}

func (c *repository) UpdateTier(ctx context.Context, id, tier string) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"tier":        tier,
		},
	}
	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err == mongo.ErrNoDocuments {
		return ErrNotFound
	}

	return err
}

func (c *repository) CreatePatientTag(ctx context.Context, id, tagName string) (*Clinic, error) {
	clinic, err := c.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagId := primitive.NewObjectID()
	tag := PatientTag{
		Id:   &tagId,
		Name: strings.TrimSpace(tagName),
	}

	canAdd, err := canAddPatientTag(*clinic, tag)
	if !canAdd {
		return nil, err
	}

	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$addToSet": bson.M{
			"patientTags": tag,
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}

	updateErr := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if updateErr == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, updateErr
	}

	return c.Get(ctx, id)
}

func (c *repository) UpdatePatientTag(ctx context.Context, id, tagId, tagName string) (*Clinic, error) {
	clinic, err := c.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagObjectId, _ := primitive.ObjectIDFromHex(tagId)
	tag := PatientTag{
		Id:   &tagObjectId,
		Name: strings.TrimSpace(tagName),
	}

	if isDuplicatePatientTag(*clinic, tag) {
		return nil, ErrDuplicatePatientTagName
	}

	clinicId, _ := primitive.ObjectIDFromHex(id)
	patientTagId, _ := primitive.ObjectIDFromHex(tagId)
	selector := bson.M{"_id": clinicId, "patientTags._id": patientTagId}

	update := bson.M{
		"$set": bson.M{
			"patientTags.$.name": strings.TrimSpace(tagName),
			"updatedTime":        time.Now(),
		},
	}

	updateErr := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if updateErr == mongo.ErrNoDocuments {
			return nil, ErrPatientTagNotFound
		}
		return nil, updateErr
	}

	return c.Get(ctx, id)
}

func (c *repository) DeletePatientTag(ctx context.Context, id, tagId string) (*Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	patientTagId, _ := primitive.ObjectIDFromHex(tagId)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$pull": bson.M{
			"patientTags": bson.M{"_id": patientTagId},
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return c.Get(ctx, id)
}

func canAddPatientTag(clinic Clinic, tag PatientTag) (bool, error) {
	if len(clinic.PatientTags) >= MaximumPatientTags {
		return false, ErrMaximumPatientTagsExceeded
	}

	if isDuplicatePatientTag(clinic, tag) {
		return false, ErrDuplicatePatientTagName
	}

	return true, nil
}

func isDuplicatePatientTag(clinic Clinic, tag PatientTag) bool {
	for _, p := range clinic.PatientTags {
		trimmedName := strings.TrimSpace(tag.Name)
		if p.Name == trimmedName {
			return true
		}
	}

	return false
}

func canRemoveAdmin(clinic Clinic, clinicianId string) bool {
	var adminsPostUpdate []string
	if clinic.Admins != nil {
		for _, admin := range *clinic.Admins {
			if admin != clinicianId {
				adminsPostUpdate = append(adminsPostUpdate, admin)
			}
		}
	}

	return len(adminsPostUpdate) >= 1
}

func setUpdatedTime(clinic *Clinic) {
	clinic.UpdatedTime = time.Now()
}

func setCreatedTime(clinic *Clinic) {
	clinic.CreatedTime = time.Now()
}

func createUpdateDocument(clinic *Clinic) bson.M {
	update := bson.M{}
	if clinic != nil {
		// Make sure we're not overriding the id and sharecodes
		clinic.Id = nil
		clinic.ShareCodes = nil

		// Refresh updatedTime timestamp
		setUpdatedTime(clinic)

		update["$set"] = clinic
		if clinic.CanonicalShareCode != nil {
			update["$addToSet"] = bson.M{
				"shareCodes": *clinic.CanonicalShareCode,
			}
		}
	}

	return update
}
