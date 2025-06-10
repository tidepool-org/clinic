package clinics

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

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

const (
	CollectionName = "clinics"
)

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Service, error) {
	deletionsRepo, err := deletions.NewRepository[Clinic]("clinic", db, logger)
	if err != nil {
		return nil, err
	}

	repo := &repository{
		collection:    db.Collection(CollectionName),
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
	deletionsRepo deletions.Repository[Clinic]
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

func (r *repository) Get(ctx context.Context, id string) (*Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	clinic := &Clinic{}
	err := r.collection.FindOne(ctx, selector).Decode(&clinic)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return clinic, nil
}

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error) {
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

	clinics := make([]*Clinic, 0)
	if err = cursor.All(ctx, &clinics); err != nil {
		return nil, fmt.Errorf("error decoding clinics list: %w", err)
	}

	return clinics, nil
}

func (r *repository) Create(ctx context.Context, clinic *Clinic) (*Clinic, error) {
	clinics, err := r.List(ctx, &Filter{ShareCodes: *clinic.ShareCodes}, store.Pagination{Limit: 1, Offset: 0})
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
	res, err := r.collection.InsertOne(ctx, clinic)
	if err != nil {
		return nil, fmt.Errorf("error creating clinic: %w", err)
	}

	id := res.InsertedID.(primitive.ObjectID)
	return r.Get(ctx, id.Hex())
}

func (r *repository) Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error) {
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
		return ErrNotFound
	}

	return err
}
func (r *repository) RemoveAdmin(ctx context.Context, id, clinicianId string, allowOrphaning bool) error {
	clinic, err := r.Get(ctx, id)
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
		return ErrNotFound
	}

	return err
}

func (r *repository) UpdateSuppressedNotifications(ctx context.Context, id string, suppressedNotifications SuppressedNotifications) error {
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
		return ErrNotFound
	}

	return err
}

func (r *repository) CreatePatientTag(ctx context.Context, id, tagName string) (*PatientTag, error) {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagId := primitive.NewObjectID()
	tag := PatientTag{
		Id:   &tagId,
		Name: strings.TrimSpace(tagName),
	}

	if err := AssertCanAddPatientTag(*clinic, tag); err != nil {
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
			return nil, ErrNotFound
		}
		return nil, updateErr
	}

	return &tag, nil
}

func (r *repository) UpdatePatientTag(ctx context.Context, id, tagId, tagName string) (*PatientTag, error) {
	clinic, err := r.Get(ctx, id)
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

	updateErr := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if errors.Is(updateErr, mongo.ErrNoDocuments) {
			return nil, ErrPatientTagNotFound
		}
		return nil, updateErr
	}

	return &tag, nil
}

func (r *repository) DeletePatientTag(ctx context.Context, id, tagId string) error {
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
			return ErrNotFound
		}
		return err
	}

	return nil
}

func (r *repository) ListMembershipRestrictions(ctx context.Context, id string) ([]MembershipRestrictions, error) {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return clinic.MembershipRestrictions, nil
}

func (r *repository) UpdateMembershipRestrictions(ctx context.Context, id string, restrictions []MembershipRestrictions) error {
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
		return ErrNotFound
	}

	return err
}

func (r *repository) GetEHRSettings(ctx context.Context, clinicId string) (*EHRSettings, error) {
	clinic, err := r.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.EHRSettings, nil
}

func (r *repository) UpdateEHRSettings(ctx context.Context, id string, settings *EHRSettings) error {
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
		return ErrNotFound
	}

	return err
}

func (r *repository) GetMRNSettings(ctx context.Context, clinicId string) (*MRNSettings, error) {
	clinic, err := r.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.MRNSettings, nil
}

func (r *repository) UpdateMRNSettings(ctx context.Context, id string, settings *MRNSettings) error {
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
		return ErrNotFound
	}

	return err
}

func (r *repository) GetPatientCountSettings(ctx context.Context, clinicId string) (*PatientCountSettings, error) {
	clinic, err := r.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.PatientCountSettings, nil
}

func (r *repository) UpdatePatientCountSettings(ctx context.Context, id string, settings *PatientCountSettings) error {
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
		return ErrNotFound
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
		return ErrNotFound
	}

	return err
}

func (r *repository) GetPatientCount(ctx context.Context, clinicId string) (*PatientCount, error) {
	clinic, err := r.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.PatientCount, nil
}

func (r *repository) UpdatePatientCount(ctx context.Context, id string, patientCount *PatientCount) error {
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
		return ErrNotFound
	}

	return err
}

// CreateSite while checking constraints.
func (c *repository) CreateSite(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error) {

	if err := c.maintainSitesConstraintsOnCreate(ctx, clinicId, site.Name); err != nil {
		return nil, err
	}
	id, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return nil, err
	}
	if site.Id.IsZero() {
		site.Id = primitive.NewObjectID()
	}
	filter := bson.M{"_id": id}
	update := bson.M{
		"$push":        bson.M{"sites": site},
		"$currentDate": bson.M{"updatedTime": bson.M{"$type": "timestamp"}},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := c.collection.FindOneAndUpdate(ctx, filter, update, opts)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	clinic := &Clinic{}
	if err := res.Decode(clinic); err != nil {
		return nil, err
	}
	for _, candidate := range clinic.Sites {
		if candidate.Name == site.Name {
			return &candidate, nil
		}
	}

	return nil, fmt.Errorf("unable to find newly created site %+v %s %+v", clinic.Sites, site.Name, clinic)
}

func (c *repository) DeleteSite(ctx context.Context, clinicId, siteId string) error {
	clinicOID, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}
	siteOID, err := primitive.ObjectIDFromHex(siteId)
	if err != nil {
		return err
	}
	selector := bson.M{
		"_id":      clinicOID,
		"sites.id": siteOID,
	}
	update := bson.M{
		"$pull":        bson.M{"sites": bson.M{"id": siteOID}},
		"$currentDate": bson.M{"updatedTime": bson.M{"$type": "timestamp"}},
	}
	res, err := c.collection.UpdateOne(ctx, selector, update)
	if err != nil {
		return err
	}
	if res.ModifiedCount != 1 {
		return ErrNotFound
	}
	return nil
}

func (c *repository) UpdateSite(ctx context.Context,
	clinicId, siteId string, site *sites.Site) (*sites.Site, error) {

	if err := c.maintainSitesConstraintsOnUpdate(ctx, clinicId, site.Name); err != nil {
		return nil, err
	}
	clinicOID, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return nil, err
	}
	siteOID, err := primitive.ObjectIDFromHex(siteId)
	if err != nil {
		return nil, err
	}
	selector := bson.M{
		"_id":      clinicOID,
		"sites.id": siteOID,
	}
	update := bson.M{
		"$set":         bson.M{"sites.$.name": site.Name},
		"$currentDate": bson.M{"updatedTime": bson.M{"$type": "timestamp"}},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := c.collection.FindOneAndUpdate(ctx, selector, update, opts)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	clinic := &Clinic{}
	if err := res.Decode(&clinic); err != nil {
		return nil, err
	}
	for _, clinicSite := range clinic.Sites {
		if clinicSite.Name == site.Name {
			return &clinicSite, nil
		}
	}

	return nil, fmt.Errorf("unable to find newly updated site %+v %s %+v", clinic.Sites, site.Name, clinic)
}

func (c *repository) maintainSitesConstraintsOnCreate(ctx context.Context,
	clinicId, name string) error {

	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return err
	}
	if sites.SiteExistsWithName(clinic.Sites, name) {
		return ErrDuplicateSiteName
	}
	if len(clinic.Sites) >= sites.MaxSitesPerClinic {
		return ErrMaximumSitesExceeded
	}
	return nil
}

func (c *repository) maintainSitesConstraintsOnUpdate(ctx context.Context,
	clinicId, name string) error {

	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return err
	}
	if sites.SiteExistsWithName(clinic.Sites, name) {
		return ErrDuplicateSiteName
	}
	return nil
}

func AssertCanAddPatientTag(clinic Clinic, tag PatientTag) error {
	if len(clinic.PatientTags) >= MaximumPatientTags {
		return ErrMaximumPatientTagsExceeded
	}

	if isDuplicatePatientTag(clinic, tag) {
		return ErrDuplicatePatientTagName
	}

	return nil
}

func isDuplicatePatientTag(clinic Clinic, tag PatientTag) bool {
	trimmedNewTagName := strings.ToLower(strings.ReplaceAll(tag.Name, " ", ""))

	for _, p := range clinic.PatientTags {
		// We only check for duplication against other tags
		if p.Id.Hex() != tag.Id.Hex() {
			trimmedExistingTagName := strings.ToLower(strings.ReplaceAll(p.Name, " ", ""))

			if trimmedExistingTagName == trimmedNewTagName {
				return true
			}
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
