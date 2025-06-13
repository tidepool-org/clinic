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

	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

const (
	CollectionName = "clinics"
)

func NewRepository(db *mongo.Database, lifecycle fx.Lifecycle) (Service, error) {
	repo := &repository{
		collection: db.Collection(CollectionName),
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
		{
			Keys: bson.D{
				{Key: "ehrSettings.sourceId", Value: 1},
				{Key: "ehrSettings.facility.name", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("UniqueEHRSourceFacility").
				SetPartialFilterExpression(bson.D{{Key: "ehrSettings.sourceId", Value: bson.M{"$exists": true}}}),
		},
	})
	return err
}

func (c *repository) Get(ctx context.Context, id string) (*Clinic, error) {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	clinic := &Clinic{}
	err := c.collection.FindOne(ctx, selector).Decode(&clinic)
	if errors.Is(err, mongo.ErrNoDocuments) {
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
	if filter.EHRSourceId != nil {
		selector["ehrSettings.sourceId"] = filter.EHRSourceId
	}
	if filter.EHRProvider != nil {
		selector["ehrSettings.provider"] = filter.EHRProvider
	}
	if filter.EHRFacilityName != nil {
		selector["ehrSettings.facility.name"] = filter.EHRFacilityName
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

func (c *repository) Delete(ctx context.Context, clinicId string) error {
	id, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}

	selector := bson.M{
		"_id": id,
	}

	err = c.collection.FindOneAndDelete(ctx, selector).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
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
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) UpdateSuppressedNotifications(ctx context.Context, id string, suppressedNotifications SuppressedNotifications) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":             time.Now(),
			"suppressedNotifications": suppressedNotifications,
		},
	}
	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
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

	updateErr := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if updateErr != nil {
		if errors.Is(updateErr, mongo.ErrNoDocuments) {
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
		if errors.Is(updateErr, mongo.ErrNoDocuments) {
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
			"updatedTime":           time.Now(),
			"lastDeletedPatientTag": patientTagId,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return c.Get(ctx, id)
}

func (c *repository) ListMembershipRestrictions(ctx context.Context, id string) ([]MembershipRestrictions, error) {
	clinic, err := c.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return clinic.MembershipRestrictions, nil
}

func (c *repository) UpdateMembershipRestrictions(ctx context.Context, id string, restrictions []MembershipRestrictions) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":            time.Now(),
			"membershipRestrictions": restrictions,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) GetEHRSettings(ctx context.Context, clinicId string) (*EHRSettings, error) {
	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.EHRSettings, nil
}

func (c *repository) UpdateEHRSettings(ctx context.Context, id string, settings *EHRSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"ehrSettings": settings,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) GetMRNSettings(ctx context.Context, clinicId string) (*MRNSettings, error) {
	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.MRNSettings, nil
}

func (c *repository) UpdateMRNSettings(ctx context.Context, id string, settings *MRNSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
			"mrnSettings": settings,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) GetPatientCountSettings(ctx context.Context, clinicId string) (*PatientCountSettings, error) {
	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.PatientCountSettings, nil
}

func (c *repository) UpdatePatientCountSettings(ctx context.Context, id string, settings *PatientCountSettings) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":          time.Now(),
			"patientCountSettings": settings,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) AppendShareCodes(ctx context.Context, id string, shareCodes []string) error {
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

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

func (c *repository) GetPatientCount(ctx context.Context, clinicId string) (*PatientCount, error) {
	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	return clinic.PatientCount, nil
}

func (c *repository) UpdatePatientCount(ctx context.Context, id string, patientCount *PatientCount) error {
	clinicId, _ := primitive.ObjectIDFromHex(id)
	selector := bson.M{"_id": clinicId}

	update := bson.M{
		"$set": bson.M{
			"updatedTime":  time.Now(),
			"patientCount": patientCount,
		},
	}

	err := c.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}

	return err
}

// CreateSite while checking constraints.
//
// This method is expected to be run in a transaction.
func (c *repository) CreateSite(ctx context.Context, clinicId string, site *sites.Site) error {
	if err := c.maintainSitesConstraints(ctx, clinicId, site.Name); err != nil {
		return err
	}
	id, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return err
	}
	if site.Id.IsZero() {
		site.Id = primitive.NewObjectID()
	}
	selector := bson.M{"_id": id}
	update := bson.M{
		"$push":        bson.M{"sites": site},
		"$currentDate": bson.M{"updatedTime": true},
	}
	res := c.collection.FindOneAndUpdate(ctx, selector, update)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}
	return nil
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
		"$currentDate": bson.M{"updatedTime": true},
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

func (c *repository) ListSites(ctx context.Context, clinicId string) ([]sites.Site, error) {
	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	return clinic.Sites, nil
}

func (c *repository) UpdateSite(ctx context.Context, clinicId, siteId string, site *sites.Site) error {
	if err := c.maintainSitesConstraints(ctx, clinicId, site.Name); err != nil {
		return err
	}
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
		"$set":         bson.M{"sites.$.name": site.Name},
		"$currentDate": bson.M{"updatedTime": true},
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

func (c *repository) maintainSitesConstraints(ctx context.Context, clinicId, name string) error {
	existingSites, err := c.ListSites(ctx, clinicId)
	if err != nil {
		return err
	}
	if sites.SiteExistsWithName(existingSites, name) {
		return ErrDuplicateSiteName
	}
	if len(existingSites) >= sites.MaxSitesPerClinic {
		return ErrMaximumSitesExceeded
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
