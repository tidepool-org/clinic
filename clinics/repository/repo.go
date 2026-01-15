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
	"github.com/tidepool-org/clinic/sites"
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
		logger:        logger,
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
	logger        *zap.SugaredLogger
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
	clinicId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	annotatedClinic, err := r.annotateClinic(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	return annotatedClinic, nil
}

func (r *repository) annotateClinics(ctx context.Context, match bson.M) (
	[]*clinics.Clinic, error) {

	pipeline := bson.A{
		bson.M{"$match": match},
		// I've broken this query up to make it (hopefully) a little more tractable. MongoDB
		// queries get crazy quick.
		lookupPatientTags,
		setPatientTagsPatients,
		lookupSites,
		setSitesPatients,
		bson.M{"$unset": bson.A{"tags_counts", "sites_counts"}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	clinicsList := []*clinics.Clinic{}
	if err := cursor.All(ctx, &clinicsList); err != nil {
		return nil, err
	}

	return clinicsList, nil
}

func (r *repository) annotateClinic(ctx context.Context, clinicId primitive.ObjectID) (
	*clinics.Clinic, error) {

	annotatedClinics, err := r.annotateClinics(ctx, bson.M{"_id": clinicId})
	if err != nil {
		return nil, err
	}
	if len(annotatedClinics) < 1 {
		return nil, clinics.ErrNotFound
	}
	if len(annotatedClinics) > 1 {
		return nil, fmt.Errorf("unable to annotate clinic: (expected 1, got %d)",
			len(annotatedClinics))
	}
	return annotatedClinics[0], nil
}

func (r *repository) annotateClinicSite(ctx context.Context,
	clinicId, siteId primitive.ObjectID) (*sites.Site, error) {

	annotatedClinic, err := r.annotateClinic(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	var annotatedSite *sites.Site
	for _, site := range annotatedClinic.Sites {
		if site.Id.Hex() == siteId.Hex() {
			annotatedSite = &site
			break
		}
	}
	if annotatedSite == nil {
		return nil, clinics.ErrSiteNotFound
	}
	return annotatedSite, nil
}

func (r *repository) annotateClinicPatientTag(ctx context.Context,
	clinicId, tagId primitive.ObjectID) (*clinics.PatientTag, error) {

	annotatedClinic, err := r.annotateClinic(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	var annotatedTag *clinics.PatientTag
	for _, tag := range annotatedClinic.PatientTags {
		if tag.Id.Hex() == tagId.Hex() {
			annotatedTag = &tag
			break
		}
	}
	if annotatedTag == nil {
		return nil, fmt.Errorf("patient tag %q in clinic %q: not found", tagId, clinicId)
	}
	return annotatedTag, nil
}

// lookupPatientTags adds the patient count for each patient tag.
var lookupPatientTags = bson.M{
	"$lookup": bson.M{
		"from":         "patients",
		"localField":   "_id",
		"foreignField": "clinicId",
		"as":           "tags_counts",
		"pipeline": bson.A{
			bson.M{"$unwind": "$tags"},
			bson.M{
				"$group": bson.M{
					"_id":      "$tags",
					"patients": bson.M{"$sum": 1},
				},
			},
		},
	},
}

// setPatientTagsPatients collapses patient counts from tags_counts back into patient_tags.
var setPatientTagsPatients = bson.M{
	"$set": bson.M{
		"patientTags": bson.M{
			"$map": bson.M{
				"input": "$patientTags",
				"in": bson.M{
					"$let": bson.M{
						"vars": bson.M{
							"tag_count": bson.M{
								"$first": bson.M{
									"$filter": bson.M{
										"input": "$tags_counts",
										"as":    "tc",
										"cond": bson.M{
											"$eq": bson.A{
												"$$this._id", "$$tc._id",
											},
										},
									},
								},
							},
						},
						"in": bson.M{
							"$mergeObjects": bson.A{
								"$$this",
								bson.M{
									"patients": "$$tag_count.patients",
								},
							},
						},
					},
				},
			},
		},
	},
}

// lookupSites adds the patient count for each site.
var lookupSites = bson.M{
	"$lookup": bson.M{
		"from":         "patients",
		"localField":   "_id",
		"foreignField": "clinicId",
		"as":           "sites_counts",
		"pipeline": bson.A{
			bson.M{"$unwind": "$sites"},
			bson.M{
				"$group": bson.M{
					"_id":      "$sites.id",
					"patients": bson.M{"$sum": 1},
				},
			},
		},
	},
}

// setSitesPatients collapses patient counts from sites_counts back into sites.
var setSitesPatients = bson.M{
	"$set": bson.M{
		"sites": bson.M{
			"$map": bson.M{
				"input": "$sites",
				"in": bson.M{
					"$let": bson.M{
						"vars": bson.M{
							"site_count": bson.M{
								"$first": bson.M{
									"$filter": bson.M{
										"input": "$sites_counts",
										"as":    "sc",
										"cond": bson.M{
											"$eq": bson.A{"$$this.id", "$$sc._id"},
										},
									},
								},
							},
						},
						"in": bson.M{
							"$mergeObjects": bson.A{
								"$$this", bson.M{"patients": "$$site_count.patients"},
							},
						},
					},
				},
			},
		},
	},
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
	if err := cursor.All(ctx, &clinics); err != nil {
		return nil, fmt.Errorf("error decoding clinics list: %w", err)
	}
	clinicIDs := []primitive.ObjectID{}
	for _, clinic := range clinics {
		clinicIDs = append(clinicIDs, *clinic.Id)
	}
	match := bson.M{"_id": bson.M{"$in": clinicIDs}}

	return r.annotateClinics(ctx, match)
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

func (r *repository) CreatePatientTag(ctx context.Context, id, tagName string) (*clinics.PatientTag, error) {
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

	return &tag, nil
}

func (r *repository) UpdatePatientTag(ctx context.Context, id, tagId, tagName string) (*clinics.PatientTag, error) {
	clinic, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	tagObjectId, err := primitive.ObjectIDFromHex(tagId)
	if err != nil {
		return nil, err
	}

	tag := clinics.PatientTag{
		Id:   &tagObjectId,
		Name: strings.TrimSpace(tagName),
	}

	if clinics.IsDuplicatePatientTag(*clinic, tag) {
		return nil, clinics.ErrDuplicatePatientTagName
	}

	patientTagId, err := primitive.ObjectIDFromHex(tagId)
	if err != nil {
		return nil, err
	}
	selector := bson.M{"_id": *clinic.Id, "patientTags._id": patientTagId}

	update := bson.M{
		"$set": bson.M{
			"patientTags.$.name": strings.TrimSpace(tagName),
			"updatedTime":        time.Now(),
		},
	}
	if _, err := r.collection.UpdateOne(ctx, selector, update); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, clinics.ErrPatientTagNotFound
		}
		return nil, err
	}

	return r.annotateClinicPatientTag(ctx, *clinic.Id, tagObjectId)
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
			return clinics.ErrNotFound
		}
		return err
	}

	return nil
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

// CreateSite while checking constraints.
func (c *repository) CreateSite(ctx context.Context, clinicId string, site *sites.Site) (
	*sites.Site, error) {

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
		"$currentDate": bson.M{"updatedTime": true},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := c.collection.FindOneAndUpdate(ctx, filter, update, opts)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, clinics.ErrNotFound
		}
		return nil, err
	}
	clinic := &clinics.Clinic{}
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

// CreateSiteIgnoringLimit will ignore MaxClinicSitesLimit.
//
// This is intended to be used only when merging two sites. In that case we want to allow
// the resulting clinic to be over the sites limit.
func (c *repository) CreateSiteIgnoringLimit(ctx context.Context, clinicId string,
	site *sites.Site) (*sites.Site, error) {

	if err := c.maintainSitesConstraintsOnCreate(ctx, clinicId, site.Name); err != nil {
		if !errors.Is(err, clinics.ErrMaximumSitesExceeded) {
			return nil, err
		}
		c.logger.Info("creating a site in excess of sites.MaxSitesPerClinic")
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
		"$currentDate": bson.M{"updatedTime": true},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := c.collection.FindOneAndUpdate(ctx, filter, update, opts)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, clinics.ErrNotFound
		}
		return nil, err
	}
	clinic := &clinics.Clinic{}
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
		"$currentDate": bson.M{"updatedTime": true},
	}
	res, err := c.collection.UpdateOne(ctx, selector, update)
	if err != nil {
		return err
	}
	if res.ModifiedCount != 1 {
		return clinics.ErrNotFound
	}
	return nil
}

func (c *repository) UpdateSite(ctx context.Context,
	clinicId, siteId string, site *sites.Site) (*sites.Site, error) {

	if err := c.maintainSitesConstraintsOnUpdate(ctx, clinicId, site.Name); err != nil {
		return nil, fmt.Errorf("checking site constraints: %w", err)
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
		"$currentDate": bson.M{"updatedTime": true},
	}
	if _, err := c.collection.UpdateOne(ctx, selector, update); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, clinics.ErrNotFound
		}
		return nil, fmt.Errorf("updating clinic %q site %q: %w", clinicId, siteId, err)
	}

	return c.annotateClinicSite(ctx, clinicOID, siteOID)
}

func (c *repository) maintainSitesConstraintsOnCreate(ctx context.Context,
	clinicId, name string) error {

	clinic, err := c.Get(ctx, clinicId)
	if err != nil {
		return err
	}
	if sites.SiteExistsWithName(clinic.Sites, name) {
		return clinics.ErrDuplicateSiteName
	}
	if len(clinic.Sites) >= sites.MaxSitesPerClinic {
		return clinics.ErrMaximumSitesExceeded
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
		return clinics.ErrDuplicateSiteName
	}
	return nil
}

func AssertCanAddPatientTag(clinic clinics.Clinic, tag clinics.PatientTag) error {
	if len(clinic.PatientTags) >= clinics.MaximumPatientTags {
		return clinics.ErrMaximumPatientTagsExceeded
	}

	if isDuplicatePatientTag(clinic, tag) {
		return clinics.ErrDuplicatePatientTagName
	}

	return nil
}

func isDuplicatePatientTag(clinic clinics.Clinic, tag clinics.PatientTag) bool {
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
