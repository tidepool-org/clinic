package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/store"
)

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (clinics.Repository, error) {
	deletionsRepo, err := deletions.NewRepository[clinics.Clinic]("clinic", db, logger)
	if err != nil {
		return nil, err
	}

	repo := &repository{
		collection:    db.Collection(clinics.CollectionName),
		deletionsRepo: deletionsRepo,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err = repo.Initialize(ctx); err != nil {
				return err
			}
			if err = repo.deletionsRepo.Initialize(ctx, []string{"_id"}); err != nil {
				return err
			}
			return nil
		},
	})

	return repo, nil
}

type repository struct {
	collection    *mongo.Collection
	deletionsRepo deletions.Repository[clinics.Clinic]
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "shareCodes", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("UniqueShareCodes"),
		},
		{
			Keys: bson.D{
				{Key: "canonicalShareCode", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("UniqueCanonicalShareCode"),
		},
	})
	return err
}

func (r *repository) Get(ctx context.Context, id string) (*clinics.Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	clinic := &clinics.Clinic{}
	err := r.collection.FindOne(ctx, selector).Decode(&clinic)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, clinics.ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinic, nil
}

func (r *repository) List(ctx context.Context, filter *clinics.Filter, pagination store.Pagination) ([]*clinics.Clinic, error) {
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
	if filter.EHRSourceId != nil {
		selector["ehrSettings.sourceId"] = filter.EHRSourceId
	}
	if filter.EHRProvider != nil {
		selector["ehrSettings.provider"] = filter.EHRProvider
	}
	if filter.EHREnabled != nil {
		selector["ehrSettings.enabled"] = filter.EHREnabled
	}
	if filter.ScheduledReportsOnUploadEnabled != nil {
		comparator := "$eq"
		if !(*filter.ScheduledReportsOnUploadEnabled) {
			comparator = "$neq"
		}
		selector["ehrSettings.scheduledReports.onUploadEnabled"] = bson.M{
			comparator: true,
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

	cursor, err := r.collection.Find(ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing clinics: %w", err)
	}

	clinics := make([]*clinics.Clinic, 0)
	if err = cursor.All(ctx, &clinics); err != nil {
		return nil, fmt.Errorf("error decoding clinics list: %w", err)
	}

	return clinics, nil
}

func (r *repository) Create(ctx context.Context, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	clinicList, err := r.List(ctx, &clinics.Filter{ShareCodes: *clinic.ShareCodes}, store.Pagination{Limit: 1, Offset: 0})
	if err != nil {
		return nil, fmt.Errorf("error finding clinic by sharecode: %w", err)
	}
	// Fail gracefully if there is a clinic with duplicate share code
	if len(clinicList) > 0 {
		return nil, clinics.ErrDuplicateShareCode
	}

	setCreatedTime(clinic)
	setUpdatedTime(clinic)

	// Insertion will fail if there are two concurrent requests, which are both
	// trying to create a clinic with the same share code
	res, err := r.collection.InsertOne(ctx, clinic)
	if err != nil {
		return nil, fmt.Errorf("error creating clinic: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID)
	return r.Get(ctx, id.Hex())
}

func (r *repository) Update(ctx context.Context, id string, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	setUpdatedTime(clinic)
	update := createUpdateDocument(clinic)
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		return nil, fmt.Errorf("error updating clinic: %w", err)
	}

	return r.Get(ctx, id)
}

func (r *repository) UpsertAdmin(ctx context.Context, id, clinicianId string) error {
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
	return r.collection.FindOneAndUpdate(ctx, selector, update).Err()
}

func (r *repository) Delete(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	clinic, err := r.Get(ctx, clinicId)
	if err != nil {
		return err
	}

	err = r.deletionsRepo.Create(ctx, *clinic, metadata)
	if err != nil {
		return nil
	}

	selector := bson.M{
		"_id": clinic.Id,
	}

	err = r.collection.FindOneAndDelete(ctx, selector).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}
func (r *repository) RemoveAdmin(ctx context.Context, id, clinicianId string, allowOrphaning bool) error {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	if !allowOrphaning && !canRemoveAdmin(*clinic, clinicianId) {
		return clinics.ErrAdminRequired
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

	if res, err := r.collection.UpdateOne(ctx, selector, update); err != nil {
		return err
	} else if res.MatchedCount != int64(1) {
		return fmt.Errorf("concurrent modification of clinic detected")
	}

	return nil
}

func (r *repository) UpdateTier(ctx context.Context, id, tier string) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"tier":        tier,
		},
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) UpdateSuppressedNotifications(ctx context.Context, id string, suppressedNotifications clinics.SuppressedNotifications) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":             time.Now(),
			"suppressedNotifications": suppressedNotifications,
		},
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) CreatePatientTag(ctx context.Context, id, tagName string) (*clinics.Clinic, error) {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagId := primitive.NewObjectID()
	tag := clinics.PatientTag{
		Id:   &tagId,
		Name: strings.TrimSpace(tagName),
	}

	if err := clinics.AssertCanAddPatientTag(*clinic, tag); err != nil {
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

	updateErr := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if errors.Is(updateErr, mongo.ErrNoDocuments) {
			return nil, clinics.ErrNotFound
		}
		return nil, updateErr
	}

	return r.Get(ctx, id)
}

func (r *repository) UpdatePatientTag(ctx context.Context, id, tagId, tagName string) (*clinics.Clinic, error) {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagObjectId, _ := primitive.ObjectIDFromHex(tagId)
	tag := clinics.PatientTag{
		Id:   &tagObjectId,
		Name: strings.TrimSpace(tagName),
	}

	if clinics.IsDuplicatePatientTag(*clinic, tag) {
		return nil, clinics.ErrDuplicatePatientTagName
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

	updateErr := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if errors.Is(updateErr, mongo.ErrNoDocuments) {
			return nil, clinics.ErrPatientTagNotFound
		}
		return nil, updateErr
	}

	return r.Get(ctx, id)
}

func (r *repository) DeletePatientTag(ctx context.Context, id, tagId string) (*clinics.Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	patientTagId, _ := primitive.ObjectIDFromHex(tagId)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$pull": bson.M{
			"patientTags": bson.M{"_id": patientTagId},
		},
		"$set": bson.M{
			"updatedTime":           time.Now(),
			"lastDeletedPatientTag": patientTagId,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, clinics.ErrNotFound
		}
		return nil, err
	}

	return r.Get(ctx, id)
}

func (r *repository) UpdateMembershipRestrictions(ctx context.Context, id string, restrictions []clinics.MembershipRestrictions) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":            time.Now(),
			"membershipRestrictions": restrictions,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) UpdateEHRSettings(ctx context.Context, id string, settings *clinics.EHRSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"ehrSettings": settings,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) UpdateMRNSettings(ctx context.Context, id string, settings *clinics.MRNSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"mrnSettings": settings,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) UpdatePatientCountSettings(ctx context.Context, id string, settings *clinics.PatientCountSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":          time.Now(),
			"patientCountSettings": settings,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) AppendShareCodes(ctx context.Context, id string, shareCodes []string) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$addToSet": bson.M{
			"shareCodes": bson.M{
				"$each": shareCodes,
			},
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}

func (r *repository) UpdatePatientCount(ctx context.Context, id string, patientCount *clinics.PatientCount) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":  time.Now(),
			"patientCount": patientCount,
		},
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return clinics.ErrNotFound
	}

	return err
}
func canRemoveAdmin(clinic clinics.Clinic, clinicianId string) bool {
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

func setUpdatedTime(clinic *clinics.Clinic) {
	clinic.UpdatedTime = time.Now()
}

func setCreatedTime(clinic *clinics.Clinic) {
	clinic.CreatedTime = time.Now()
}

func createUpdateDocument(clinic *clinics.Clinic) bson.M {
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
