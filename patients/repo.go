package patients

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tidepool-org/clinic/config"
	errors2 "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	CollectionName = "patients"
)

// Collation to use for string fields
var collation = options.Collation{Locale: "en", Strength: 1}

//go:generate mockgen --build_flags=--mod=mod -source=./repo.go -destination=./test/mock_repository.go -package test -aux_files=github.com/tidepool-org/clinic/patients=patients.go MockRepository

type Repository interface {
	Service
}

func NewRepository(config *config.Config, db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository, error) {
	repo := &repository{
		config:     config,
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

type repository struct {
	config     *config.Config
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
			},
			Options: options.Index().
				SetName("UserId"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "userId", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("UniquePatient"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "fullName", Value: 1},
			},
			Options: options.Index().
				SetName("PatientFullNameEn").
				SetCollation(&collation),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "birthDate", Value: 1},
			},
			Options: options.Index().
				SetName("PatientBirthDate"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "email", Value: 1},
			},
			Options: options.Index().
				SetName("PatientEmail"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "mrn", Value: 1},
			},
			Options: options.Index().
				SetName("PatientMRN"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "userId", Value: 1},
				{Key: "tags", Value: 1},
			},
			Options: options.Index().
				SetName("PatientTagsV2"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "dataSources.providerName", Value: 1},
			},
			Options: options.Index().
				SetName("DataSourcesProviderName"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "dataSources.state", Value: 1},
			},
			Options: options.Index().
				SetName("DataSourcesState"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "mrn", Value: 1},
				// The field is not used and only set here to allow the creation of
				// a second index on (clinicId, mrn) but with different options
				{Key: "_na", Value: 1},
			},
			Options: options.Index().
				// Enforce unique constraints when the MRN is present and when uniqueness is enabled
				// for the patient of the clinic (based on the clinic settings)
				SetName("UniqueMrn").
				SetUnique(true).
				SetPartialFilterExpression(bson.D{
					{"mrn", bson.M{"$type": "string"}},
					{"requireUniqueMrn", bson.M{"$eq": true}},
				}),
		},
	})
	return err
}

func (r *repository) Get(ctx context.Context, clinicId string, userId string) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   userId,
	}

	patient := &Patient{}
	err := r.collection.FindOne(ctx, selector).Decode(&patient)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return patient, nil
}

func (r *repository) Remove(ctx context.Context, clinicId string, userId string, _ *string) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   userId,
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

func (r *repository) Count(ctx context.Context, filter *Filter) (int, error) {
	count, err := r.collection.CountDocuments(ctx, r.generateListFilterQuery(filter))
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination, sorts []*store.Sort) (*ListResult, error) {
	// We use an aggregation pipeline with facet in order to get the count
	// and the patients from a single query
	pipeline := []bson.M{
		{"$match": r.generateListFilterQuery(filter)},
	}
	if filter.ExcludeSummaryExceptFieldsInMergeReports {
		pipeline = append(pipeline, excludeSummaryExceptFieldsInMergeReports()...)
	}
	pipeline = append(pipeline, bson.M{"$sort": generateListSortStage(sorts)})
	pipeline = append(pipeline, generatePaginationFacetStages(pagination)...)

	hasFullNameSort := false
	for _, sort := range sorts {
		if sort.Attribute == "fullName" {
			hasFullNameSort = true
		}
	}

	var opts *options.AggregateOptions
	if len(sorts) == 0 || hasFullNameSort {
		// Case-insensitive sorting when sorting by fullName
		opts = options.Aggregate().SetCollation(&collation)
	}

	r.logger.Debugw("retrieving list of patients", "pipeline", pipeline)

	cursor, err := r.collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing patients: %w", err)
	}
	if !cursor.Next(ctx) {
		return nil, fmt.Errorf("error getting pipeline result")
	}

	result := ListResult{}
	if err = cursor.Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding patients list: %w", err)
	}

	if result.MatchingCount == 0 {
		result.Patients = make([]*Patient, 0)
	}

	return &result, nil
}

// excludeSummaryExceptFieldsInMergeReports from a MongoDB aggregation pipeline.
//
// For when you don't want the entire summary to be included in the result, but the last
// updated date is expected to be used when generating clinic merge reports.
func excludeSummaryExceptFieldsInMergeReports() []bson.M {
	out := []bson.M{}
	out = append(out, bson.M{"$addFields": bson.M{
		"__tmp_cgm__": "$summary.cgmStats.dates",
		"__tmp_bgm__": "$summary.bgmStats.dates",
	}})
	out = append(out, bson.M{"$unset": "summary"})
	out = append(out, bson.M{"$addFields": bson.M{
		"summary.cgmStats.dates": "$__tmp_cgm__",
		"summary.bgmStats.dates": "$__tmp_bgm__",
	}})
	out = append(out, bson.M{"$unset": []string{"__tmp_bgm__", "__tmp_cgm__"}})
	return out
}

func (r *repository) Create(ctx context.Context, patient Patient) (*Patient, error) {
	if patient.ClinicId == nil {
		return nil, fmt.Errorf("patient clinic id is missing")
	}
	if patient.UserId == nil {
		return nil, fmt.Errorf("patient user id is missing")
	}

	clinicId := patient.ClinicId.Hex()
	filter := &Filter{
		ClinicId: &clinicId,
		UserId:   patient.UserId,
	}
	patients, err := r.List(ctx, filter, store.Pagination{Limit: 1}, nil)
	if err != nil {
		return nil, fmt.Errorf("error checking for duplicate PatientsRepo: %v", err)
	}

	if patients.MatchingCount > 0 {
		if len(patient.LegacyClinicianIds) == 0 {
			return nil, ErrDuplicatePatient
		}
		// The user is being migrated multiple times from different legacy clinician accounts
		if err = r.updateLegacyClinicianIds(ctx, patient); err != nil {
			return nil, err
		}
	} else {
		patient.CreatedTime = time.Now()
		patient.UpdatedTime = time.Now()
		if _, err = r.collection.InsertOne(ctx, patient); err != nil {
			return nil, fmt.Errorf("error creating patient: %w", err)
		}
	}

	result, err := r.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
	return result, err
}

func (r *repository) Update(ctx context.Context, patientUpdate PatientUpdate) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(patientUpdate.ClinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   patientUpdate.UserId,
	}

	patient := patientUpdate.Patient
	patient.UpdatedTime = time.Now()

	update := bson.M{
		"$set": patient,
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, patientUpdate.ClinicId, patientUpdate.UserId)
}

func (r *repository) UpdateEmail(ctx context.Context, userId string, email *string) error {
	selector := bson.M{
		"userId": userId,
	}

	update := bson.M{}
	if email == nil || *email == "" {
		update["$unset"] = bson.M{
			"email": "",
		}
		update["$set"] = bson.M{
			"updatedTime": time.Now(),
		}
	} else {
		update["$set"] = bson.M{
			"email":       *email,
			"updatedTime": time.Now(),
		}
	}

	result, err := r.collection.UpdateMany(ctx, selector, update)
	if result != nil && result.MatchedCount > 0 && result.MatchedCount > result.ModifiedCount {
		err = fmt.Errorf("partially updated %v out of %v patient records: %w", result.ModifiedCount, result.MatchedCount, err)
	}
	if err != nil {
		r.logger.Errorw("error updating patient emails", "error", err, "userId", userId)
	}

	return err
}

func (r *repository) AddReview(ctx context.Context, clinicId, userId string, review Review) ([]Review, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   userId,
	}

	update := bson.M{
		"$push": bson.M{
			"reviews": bson.M{
				"$each":     []Review{review},
				"$position": 0,
				"$slice":    2,
			},
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	patient := Patient{}
	err := r.collection.FindOneAndUpdate(ctx, selector, update, opts).Decode(&patient)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return patient.Reviews, nil
}

func (r *repository) DeleteReview(ctx context.Context, clinicId, clinicianId, userId string) ([]Review, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId":              clinicObjId,
		"userId":                userId,
		"reviews.0.clinicianId": clinicianId,
	}

	update := bson.M{"$pop": bson.M{"reviews": -1}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	opts.SetProjection(bson.M{"reviews": 1})

	patient := Patient{}
	err := r.collection.FindOneAndUpdate(ctx, selector, update, opts).Decode(&patient)
	if err != nil {
		// This checking is after the fact to avoid get-modify-update race
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = r.collection.FindOne(ctx, bson.M{"clinicId": clinicObjId, "userId": userId}).Decode(&patient)
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, ErrNotFound
				}
				return nil, err
			}

			if patient.Reviews[0].ClinicianId != clinicianId {
				return nil, ErrReviewNotOwner
			}

		}
		return nil, err
	}
	return patient.Reviews, nil
}

func (r *repository) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   userId,
	}

	update := bson.M{}
	if permissions == nil {
		update["$unset"] = bson.M{
			"permissions": "",
		}
		update["$set"] = bson.M{
			"updatedTime": time.Now(),
		}
	} else {
		update["$set"] = bson.M{
			"permissions": permissions,
			"updatedTime": time.Now(),
		}
	}

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
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
		"userId":   userId,
		key:        bson.M{"$exists": true},
	}

	update := bson.M{
		"$unset": bson.D{{Key: key, Value: ""}},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrPermissionNotFound
		}
		return nil, fmt.Errorf("error removing permission: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}

func (r *repository) DeleteFromAllClinics(ctx context.Context, userId string) ([]string, error) {
	selector := bson.M{
		"userId": userId,
	}

	cursor, err := r.collection.Find(ctx, selector)
	if err != nil {
		return nil, fmt.Errorf("error listing patients: %w", err)
	}

	var patients []*Patient
	if err = cursor.All(ctx, &patients); err != nil {
		return nil, fmt.Errorf("error decoding patients list: %w", err)
	}

	var clinicIds []string
	for _, patient := range patients {
		selector["clinicId"] = patient.ClinicId
		result, err := r.collection.DeleteOne(ctx, selector)
		if err != nil {
			return clinicIds, fmt.Errorf("error deleting patient: %w", err)
		} else if result.DeletedCount >= 0 {
			clinicIds = append(clinicIds, patient.ClinicId.Hex())
		}
	}
	return clinicIds, nil
}

func (r *repository) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) (bool, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"$or": []bson.M{
			{"permissions.custodian": bson.M{"$exists": false}},
			{"permissions.custodian": bson.M{"$eq": false}},
		},
	}

	result, err := r.collection.DeleteMany(ctx, selector)
	return result.DeletedCount > 0, err
}

func (r *repository) UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *Summary) error {
	selector := bson.M{
		"userId": userId,
	}

	set := bson.M{}
	unset := bson.M{}
	if summary == nil {
		unset = bson.M{
			"summary": "",
		}
	} else {
		if summary.CGM != nil {
			set["summary.cgmStats"] = summary.CGM
		}
		if summary.BGM != nil {
			set["summary.bgmStats"] = summary.BGM
		}
	}

	update := bson.M{}
	if len(set) > 0 {
		update["$set"] = set
	}
	if len(unset) > 0 {
		update["$unset"] = unset
	}

	res, err := r.collection.UpdateMany(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error updating patient: %w", err)
	} else if res.ModifiedCount == 0 {
		return SummaryNotFound
	}

	return nil
}

func (r *repository) DeleteSummaryInAllClinics(ctx context.Context, summaryId string) error {
	// we dont know which type the summary Id is from, so we must try deleting both.
	selectorCgm := bson.M{
		"summary.cgmStats.id": summaryId,
	}
	updateCgm := bson.M{"$unset": bson.M{"summary.cgmStats": ""}}

	selectorBgm := bson.M{
		"summary.bgmStats.id": summaryId,
	}
	updateBgm := bson.M{"$unset": bson.M{"summary.bgmStats": ""}}

	resCgm, err := r.collection.UpdateMany(ctx, selectorCgm, updateCgm)
	if err != nil {
		return fmt.Errorf("error removing cgmStats from patient: %w", err)
	}

	resBgm, err := r.collection.UpdateMany(ctx, selectorBgm, updateBgm)
	if err != nil {
		return fmt.Errorf("error removing bgmStats from patient: %w", err)
	}

	if resCgm.ModifiedCount == 0 && resBgm.ModifiedCount == 0 {
		return SummaryNotFound
	}

	return nil
}

func (r *repository) UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(update.ClinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"userId":   update.UserId,
	}

	mongoUpdate := bson.M{
		"$set": bson.M{
			"lastUploadReminderTime": update.Time,
			"updatedTime":            time.Now(),
		},
	}
	err := r.collection.FindOneAndUpdate(ctx, selector, mongoUpdate).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, update.ClinicId, update.UserId)
}

func (r *repository) AddProviderConnectionRequest(ctx context.Context, clinicId, userId string, request ConnectionRequest) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	currentTime := time.Now()

	// We fetch the current dexcom data source to determine if we are requesting an initial connection
	// or a reconnection to a previously connected data source, which will have a `ModifiedTime` set
	patient, err := r.Get(ctx, clinicId, userId)
	if err != nil {
		return fmt.Errorf("error finding patient: %w", err)
	}

	var providerDataSource DataSource
	if patient.DataSources != nil {
		for _, source := range *patient.DataSources {
			if source.ProviderName == request.ProviderName {
				providerDataSource = source
			}
		}
	}

	selector := bson.M{
		"clinicId":                 clinicObjId,
		"userId":                   userId,
		"dataSources.providerName": request.ProviderName,
	}

	// Default update for initial connection requests
	mongoUpdate := bson.M{
		"$set": bson.M{
			"updatedTime":                  currentTime,
			"dataSources.$.expirationTime": currentTime.Add(PendingDataSourceExpirationDuration),
			"dataSources.$.state":          DataSourceStatePending,
		},
	}

	// Update for previously connected requests
	if providerDataSource.ModifiedTime != nil {
		mongoUpdate = bson.M{
			"$set": bson.M{
				"updatedTime":                  currentTime,
				"dataSources.$.expirationTime": currentTime.Add(PendingDataSourceExpirationDuration),
				"dataSources.$.modifiedTime":   currentTime,
				"dataSources.$.state":          DataSourceStatePendingReconnect,
			},
		}
	}

	key := "providerConnectionRequests." + request.ProviderName
	mongoUpdate["$push"] = bson.M{
		key: bson.M{
			"$each": bson.A{request},
			// Prepend, so the most recent request is stored first
			"$position": 0,
		},
	}

	err = r.collection.FindOneAndUpdate(ctx, selector, mongoUpdate).Err()
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return fmt.Errorf("error updating patient: %w", err)
	}

	return nil
}

func (r *repository) RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error {
	params := RescheduleOrderPipelineParams{
		clinicIds:        []string{clinicId},
		subscription:     subscription,
		ordersCollection: ordersCollection,
		targetCollection: targetCollection,
	}
	pipeline := reschedulePipeline(params)
	_, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("error rescheduling subscription %s for clinic %s: %w", subscription, clinicId, err)
	}

	return nil
}

func (r *repository) RescheduleLastSubscriptionOrderForPatient(ctx context.Context, clinicIds []string, userId, subscription, ordersCollection, targetCollection string) error {
	if len(clinicIds) == 0 {
		return nil
	}

	params := RescheduleOrderPipelineParams{
		clinicIds:        clinicIds,
		userId:           &userId,
		subscription:     subscription,
		ordersCollection: ordersCollection,
		targetCollection: targetCollection,
	}

	pipeline := reschedulePipeline(params)
	_, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("error rescheduling subscription %s for patient %s (clinics: %s): %w", subscription, userId, strings.Join(clinicIds, ", "), err)
	}

	return nil
}

func (r *repository) updateLegacyClinicianIds(ctx context.Context, patient Patient) error {
	selector := bson.M{
		"clinicId": patient.ClinicId,
		"userId":   patient.UserId,
	}
	update := bson.M{
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
		"$addToSet": bson.M{
			"legacyClinicianIds": bson.M{
				"$each": patient.LegacyClinicianIds,
			},
		},
	}
	res, err := r.collection.UpdateOne(ctx, selector, update)
	if err != nil {
		return err
	} else if res.ModifiedCount == 0 {
		return fmt.Errorf("unable to update legacy clinician ids")
	}
	return nil
}

func (r *repository) AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	patientTagId, _ := primitive.ObjectIDFromHex(tagId)

	// We can't $addToSet below for any patients with a tags field value of `null`,
	// so we set the field to an empty array if that's the case
	tagsFieldisNullSelector := bson.M{
		"clinicId": clinicObjId,
		"tags":     bson.M{"$type": bson.TypeNull},
	}
	// Apply the tag to all patients if the slice is EXPLICITLY nil (and not just empty)
	if patientIds != nil {
		tagsFieldisNullSelector["userId"] = bson.M{"$in": patientIds}
	}

	tagsFieldisNullUpdate := bson.M{
		"$set": bson.M{
			"tags": bson.A{},
		},
	}

	_, arraySetErr := r.collection.UpdateMany(ctx, tagsFieldisNullSelector, tagsFieldisNullUpdate)
	if arraySetErr != nil {
		return fmt.Errorf("error ensuring patient tags field is an array: %w", arraySetErr)
	}

	selector := bson.M{
		"clinicId": clinicObjId,
		"tags":     bson.M{"$nin": bson.A{patientTagId}},
	}
	// Apply the tag to all patients if the slice is EXPLICITLY nil (and not just empty)
	if patientIds != nil {
		selector["userId"] = bson.M{"$in": patientIds}
	}

	update := bson.M{
		"$addToSet": bson.M{
			"tags": patientTagId,
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}

	_, err := r.collection.UpdateMany(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error assigning patient tag to patients: %w", err)
	}

	return nil
}

func (r *repository) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	patientTagId, _ := primitive.ObjectIDFromHex(tagId)

	selector := bson.M{
		"clinicId": clinicObjId,
		"tags":     patientTagId,
	}

	if patientIds != nil {
		selector["userId"] = bson.M{"$in": patientIds}
	}

	update := bson.M{
		"$pull": bson.M{
			"tags": patientTagId,
		},
		"$set": bson.M{
			"updatedTime": time.Now(),
		},
	}

	_, err := r.collection.UpdateMany(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error removing patient tag from patients: %w", err)
	}

	return nil
}

func (r *repository) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error {
	selector := bson.M{
		"userId": userId,
	}

	update := bson.M{
		"$set": bson.M{
			"dataSources": dataSources,
			"updatedTime": time.Now(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, selector, update)
	if result != nil && result.MatchedCount > 0 && result.MatchedCount > result.ModifiedCount {
		err = fmt.Errorf("partially updated %v out of %v patient records: %w", result.ModifiedCount, result.MatchedCount, err)
	}
	if err != nil {
		r.logger.Errorw("error updating patient data sources", "error", err, "userId", userId)
	}

	return nil
}

func (r *repository) UpdateEHRSubscription(ctx context.Context, clinicId, patientId string, update SubscriptionUpdate) error {
	patient, err := r.Get(ctx, clinicId, patientId)
	if err != nil {
		return err
	}

	if patient.UserId == nil || *patient.UserId == "" || patient.ClinicId == nil {
		return fmt.Errorf("patient is missing required fields")
	}

	selector := bson.M{
		"userId":      patient.UserId,
		"clinicId":    patient.ClinicId,
		"updatedTime": patient.UpdatedTime,
	}

	subscriptions := patient.EHRSubscriptions
	if subscriptions == nil {
		subscriptions = make(map[string]EHRSubscription)
	}

	now := time.Now()
	subscription, ok := subscriptions[update.Name]
	if !ok {
		subscription = EHRSubscription{
			CreatedAt: now,
		}
	}

	subscription.Active = update.Active
	subscription.MatchedMessages = append(subscription.MatchedMessages, update.MatchedMessage)
	subscription.Provider = update.Provider
	subscription.UpdatedAt = now
	subscriptions[update.Name] = subscription

	res, err := r.collection.UpdateOne(ctx, selector, bson.M{
		"$set": bson.M{
			"ehrSubscriptions": subscriptions,
		},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("no patient found to update")
	}
	return nil
}

func (r *repository) generateListFilterQuery(filter *Filter) bson.M {
	selector := bson.M{}
	if filter.ClinicId != nil {
		clinicId := *filter.ClinicId
		clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
		selector["clinicId"] = clinicObjId
	}
	if filter.ClinicIds != nil {
		clinicObjIds := make([]primitive.ObjectID, len(filter.ClinicIds))
		for _, clinicId := range filter.ClinicIds {
			if clinicObjId, err := primitive.ObjectIDFromHex(clinicId); err == nil {
				clinicObjIds = append(clinicObjIds, clinicObjId)
			}
		}
		selector["clinicId"] = bson.M{
			"$in": clinicObjIds,
		}
	}
	userIdSelector := bson.M{}
	if filter.UserId != nil {
		userIdSelector["$eq"] = filter.UserId
	}
	if filter.ExcludeDemo {
		userIdSelector["$ne"] = r.config.ClinicDemoPatientUserId
	}
	if len(userIdSelector) > 0 {
		selector["userId"] = userIdSelector
	}
	if filter.Mrn != nil {
		selector["mrn"] = filter.Mrn
	} else if filter.HasMRN != nil {
		empty := bson.M{
			"$in": bson.A{nil, ""},
		}
		if *filter.HasMRN {
			selector["mrn"] = bson.M{
				"$not": empty,
			}
		} else {
			selector["mrn"] = empty
		}
	}
	if filter.HasSubscription != nil {
		empty := bson.M{
			"$in": bson.A{nil, bson.M{}},
		}
		if *filter.HasSubscription {
			selector["ehrSubscriptions"] = bson.M{
				"$not": empty,
			}
		} else {
			selector["ehrSubscriptions"] = empty
		}
	}
	if filter.BirthDate != nil {
		selector["birthDate"] = filter.BirthDate
	}
	if filter.FullName != nil {
		selector["fullName"] = filter.FullName
	}
	if filter.Search != nil {
		search := regexp.QuoteMeta(*filter.Search)
		filter := bson.M{"$regex": primitive.Regex{
			Pattern: search,
			Options: "i",
		}}
		selector["$or"] = bson.A{
			bson.M{"fullName": filter},
			bson.M{"email": filter},
			bson.M{"mrn": filter},
			bson.M{"birthDate": filter},
		}
	}

	if filter.Tags != nil {
		selector["tags"] = bson.M{
			"$all": store.ObjectIDSFromStringArray(*filter.Tags),
		}
	}

	if filter.LastReviewed != nil {
		selector["reviews.0.time"] = bson.M{"$lte": filter.LastReviewed}
	}

	for field, pair := range filter.CGM {
		MaybeApplyNumericFilter(selector,
			*filter.Period,
			"cgm",
			field,
			pair,
		)
	}

	for field, pair := range filter.BGM {
		MaybeApplyNumericFilter(selector,
			*filter.Period,
			"bgm",
			field,
			pair,
		)
	}

	for field, pair := range filter.CGMTime {
		ApplyDateFilter(selector,
			"cgm",
			field,
			pair,
		)
	}

	for field, pair := range filter.BGMTime {
		ApplyDateFilter(selector,
			"bgm",
			field,
			pair,
		)
	}

	return selector
}

func MaybeApplyNumericFilter(selector bson.M, period string, typ string, field string, pair FilterPair) {
	if operator, ok := cmpToMongoFilter(&pair.Cmp); ok {
		selector["summary."+typ+"Stats.periods."+period+"."+field] = bson.M{operator: pair.Value}
	}
}

func ApplyDateFilter(selector bson.M, typ string, field string, pair FilterDatePair) {
	dateFilter := bson.M{}

	if pair.Min != nil {
		dateFilter["$gte"] = pair.Min
	}

	if pair.Max != nil {
		dateFilter["$lt"] = pair.Max
	}

	selector["summary."+typ+"Stats.dates."+field] = dateFilter
}

func generateListSortStage(sorts []*store.Sort) bson.D {
	var s bson.D
	idSortExists := false

	for _, sort := range sorts {
		if sort != nil {
			if sort.Attribute == "_id" {
				idSortExists = true
			}
			s = append(s, bson.E{Key: sort.Attribute, Value: sort.Order()})
		}
	}

	if len(s) == 0 {
		s = append(s, bson.E{Key: "fullName", Value: 1})
	}

	// Including _id in the sort query ensures that $skip aggregation works correctly
	// See https://docs.mongodb.com/manual/reference/operator/aggregation/skip/
	// for more details
	if !idSortExists {
		s = append(s, bson.E{Key: "_id", Value: 1})
	}
	return s
}

func generatePaginationFacetStages(pagination store.Pagination) []bson.M {
	return []bson.M{
		{
			"$facet": bson.M{
				"data": []bson.M{
					{"$match": bson.M{}},
					{"$skip": pagination.Offset},
					{"$limit": pagination.Limit},
				},
				"meta": []bson.M{
					{"$count": "count"},
				},
			},
		},
		// The facet above returns the count in an object as first element of the array, e.g.:
		// {
		//   "data": [],
		//   "meta": [{"count": 1}]
		// }
		// The projections below lifts it up to the top level, e.g.:
		// {
		//   "data": [],
		//   "count": 1,
		// }
		{
			"$project": bson.M{
				"data": "$data",
				"temp_count": bson.M{
					"$arrayElemAt": bson.A{"$meta", 0},
				},
			},
		},
		{
			"$project": bson.M{
				"data":  "$data",
				"count": "$temp_count.count",
			},
		},
	}
}

var cmpToFilter = map[string]string{
	">":  "$gt",
	">=": "$gte",
	"<":  "$lt",
	"<=": "$lte",
}

func cmpToMongoFilter(cmp *string) (string, bool) {
	if cmp == nil {
		return "", false
	}

	f, ok := cmpToFilter[*cmp]
	return f, ok
}

func PatientsToTideResult(patientsList []*Patient, period string, exclusions *[]primitive.ObjectID) []TideResultPatient {
	categoryResult := make([]TideResultPatient, 0, 25)
	for _, patient := range patientsList {
		*exclusions = append(*exclusions, *patient.Id)

		var patientTags []string
		for _, tag := range *patient.Tags {
			patientTags = append(patientTags, tag.Hex())
		}

		resultPatient := TideResultPatient{
			Patient: TidePatient{
				Email:       patient.Email,
				FullName:    patient.FullName,
				Id:          patient.UserId,
				Tags:        patientTags,
				Reviews:     patient.Reviews,
				DataSources: patient.DataSources,
			},
		}
		if patient.Summary != nil && patient.Summary.CGM != nil {
			resultPatient.LastData = patient.Summary.CGM.Dates.LastData

			if v, ok := patient.Summary.CGM.Periods[period]; ok {
				resultPatient.AverageGlucoseMmol = v.AverageGlucoseMmol
				resultPatient.GlucoseManagementIndicator = v.GlucoseManagementIndicator
				resultPatient.TimeCGMUseMinutes = v.TimeCGMUseMinutes
				resultPatient.TimeCGMUsePercent = v.TimeCGMUsePercent
				resultPatient.TimeInHighPercent = v.TimeInHighPercent
				resultPatient.TimeInLowPercent = v.TimeInLowPercent
				resultPatient.TimeInTargetPercent = v.TimeInTargetPercent
				resultPatient.TimeInTargetPercentDelta = v.TimeInTargetPercentDelta
				resultPatient.TimeInVeryHighPercent = v.TimeInVeryHighPercent
				resultPatient.TimeInVeryLowPercent = v.TimeInVeryLowPercent
				resultPatient.TimeInAnyLowPercent = v.TimeInAnyLowPercent
				resultPatient.TimeInAnyHighPercent = v.TimeInAnyHighPercent
			}
		}

		categoryResult = append(categoryResult, resultPatient)
	}

	return categoryResult
}

func (r *repository) TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error) {
	if clinicId == "" {
		return nil, fmt.Errorf("%w: empty clinicId provided", errors2.BadRequest)
	}
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)

	if params.Tags == nil || len(*params.Tags) < 1 {
		return nil, fmt.Errorf("%w: no tags provided", errors2.BadRequest)
	}
	tags := store.ObjectIDSFromStringArray(*params.Tags)

	if params.LastDataCutoff == nil || params.LastDataCutoff.IsZero() {
		return nil, fmt.Errorf("%w: no lastDataCutoff provided", errors2.BadRequest)
	}

	if params.Period == nil {
		return nil, fmt.Errorf("%w: no period provided", errors2.BadRequest)
	}

	if *params.Period != "1d" && *params.Period != "7d" && *params.Period != "14d" && *params.Period != "30d" {
		return nil, fmt.Errorf("%w: provided period is not one of the valid periods", errors2.BadRequest)
	}

	type Category struct {
		Heading    string
		Field      string
		Comparison string
		Value      float64
	}

	categories := [...]Category{
		{
			Heading:    "timeInVeryLowPercent",
			Field:      "timeInVeryLowPercent",
			Comparison: "$gt",
			Value:      0.01,
		},
		{
			Heading:    "timeInAnyLowPercent",
			Field:      "timeInAnyLowPercent",
			Comparison: "$gt",
			Value:      0.04,
		},
		{
			Heading:    "dropInTimeInTargetPercent",
			Field:      "timeInTargetPercentDelta",
			Comparison: "$lt",
			Value:      -0.15,
		},
		{
			Heading:    "timeInTargetPercent",
			Field:      "timeInTargetPercent",
			Comparison: "$lt",
			Value:      0.7,
		},
		{
			Heading:    "timeCGMUsePercent",
			Field:      "timeCGMUsePercent",
			Comparison: "$lt",
			Value:      0.7,
		},
	}

	limit := 50
	exclusions := make([]primitive.ObjectID, 0, 50)
	tide := Tide{
		Config: TideConfig{
			ClinicId: clinicId,
			Filters: TideFilters{
				TimeInVeryLowPercent:      ">0.01",
				TimeInAnyLowPercent:       ">0.04",
				DropInTimeInTargetPercent: "<-0.15",
				TimeInTargetPercent:       "<0.7",
				TimeCGMUsePercent:         "<0.7",
			},
			HighGlucoseThreshold:     10.0,
			LastDataCutoff:           *params.LastDataCutoff,
			LowGlucoseThreshold:      3.9,
			Period:                   *params.Period,
			SchemaVersion:            1,
			Tags:                     *params.Tags,
			VeryHighGlucoseThreshold: 13.9,
			VeryLowGlucoseThreshold:  3.0,
		},
		Results: TideResults{},
	}

	for _, category := range categories {
		selector := bson.M{
			"_id":      bson.M{"$nin": exclusions},
			"clinicId": clinicObjId,
			"tags":     bson.M{"$all": tags},
			"summary.cgmStats.periods." + *params.Period + "." + category.Field: bson.M{category.Comparison: category.Value},
			"summary.cgmStats.dates.lastData":                                   bson.M{"$gte": params.LastDataCutoff},
		}

		opts := options.Find()
		opts.SetLimit(int64(limit))

		if category.Comparison == "$gt" {
			opts.SetSort(bson.D{
				{"summary.cgmStats.periods." + *params.Period + "." + category.Field, -1},
			})
		} else {
			opts.SetSort(bson.D{
				{"summary.cgmStats.periods." + *params.Period + "." + category.Field, 1},
			})
		}

		cursor, err := r.collection.Find(ctx, selector, opts)
		if err != nil {
			return nil, err
		}

		var patientsList []*Patient
		if err = cursor.All(ctx, &patientsList); err != nil {
			return nil, fmt.Errorf("error decoding patients list: %w", err)
		}

		tide.Results[category.Heading] = PatientsToTideResult(patientsList, *params.Period, &exclusions)

		limit -= len(patientsList)
		if limit < 1 {
			break
		}
	}

	if limit > 0 {
		selector := bson.M{
			"_id":                             bson.M{"$nin": exclusions},
			"clinicId":                        clinicObjId,
			"tags":                            bson.M{"$all": tags},
			"summary.cgmStats.dates.lastData": bson.M{"$gte": params.LastDataCutoff},
		}

		opts := options.Find()
		opts.SetLimit(int64(limit))

		opts.SetSort(bson.D{
			{"summary.cgmStats.periods." + *params.Period + ".timeInTargetPercent", -1},
		})

		cursor, err := r.collection.Find(ctx, selector, opts)
		if err != nil {
			return nil, err
		}

		var patientsList []*Patient
		if err = cursor.All(ctx, &patientsList); err != nil {
			return nil, fmt.Errorf("error decoding patients list: %w", err)
		}

		tide.Results["meetingTargets"] = PatientsToTideResult(patientsList, *params.Period, &exclusions)
	}

	{
		// This specifically catches users who:
		// 1. Have no data within the last 8h
		// AND either of the following:
		// 2. Have no data within the cutoff, typically the period length being looked at, subtracted from now
		// 3. Have a dexcom session, and it is not successfully connected
		// 4. Have never had cgm data, resulting in a missing lastData field
		selector := bson.M{
			"clinicId": clinicObjId,
			"tags":     bson.M{"$all": tags},
			"$and": bson.A{
				bson.M{"$or": bson.A{
					bson.M{"summary.cgmStats.dates.lastData": bson.M{"$lt": time.Now().UTC().Add(-8 * time.Hour)}},
					bson.M{"summary.cgmStats.dates.lastData": nil},
				}},
				bson.M{"$or": bson.A{
					bson.M{"summary.cgmStats.dates.lastData": bson.M{"$lt": params.LastDataCutoff}},
					bson.M{"summary.cgmStats.dates.lastData": nil},
					bson.M{"dataSources": bson.M{"$elemMatch": bson.M{"providerName": "dexcom", "state": bson.M{"$ne": "connected"}}}},
				}},
			},
		}

		opts := options.Find()
		opts.SetLimit(int64(25))

		opts.SetSort(bson.D{
			{"summary.cgmStats.dates.lastData", 1},
		})

		cursor, err := r.collection.Find(ctx, selector, opts)
		if err != nil {
			return nil, err
		}

		var patientsList []*Patient
		if err = cursor.All(ctx, &patientsList); err != nil {
			return nil, fmt.Errorf("error decoding patients list: %w", err)
		}

		tide.Results["noData"] = PatientsToTideResult(patientsList, *params.Period, &exclusions)

	}

	return &tide, nil
}

type RescheduleOrderPipelineParams struct {
	clinicIds        []string
	userId           *string
	subscription     string
	ordersCollection string
	targetCollection string
}

func reschedulePipeline(params RescheduleOrderPipelineParams) []bson.M {
	now := time.Now()
	activeSubscriptionKey := fmt.Sprintf("ehrSubscriptions.%s.active", params.subscription)
	matchedMessagesSubscriptionKey := fmt.Sprintf("$ehrSubscriptions.%s.matchedMessages", params.subscription)

	clinicObjIds := make([]primitive.ObjectID, 0, len(params.clinicIds))
	for _, clinicId := range params.clinicIds {
		clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
		clinicObjIds = append(clinicObjIds, clinicObjId)
	}
	match := bson.M{
		"clinicId": bson.M{
			"$in": clinicObjIds,
		},
		activeSubscriptionKey: true,
	}
	if params.userId != nil {
		match["userId"] = params.userId
	}

	pipeline := []bson.M{
		{
			// Match patients with an active subscription
			"$match": match,
		},
		{
			// Extract the last matched order for the patient
			"$addFields": bson.M{
				"lastMatchedOrderRef": bson.M{
					"$arrayElemAt": bson.A{matchedMessagesSubscriptionKey, -1},
				},
			},
		},
		{
			// Get the order from the orders collection
			"$lookup": bson.M{
				"from":         params.ordersCollection,
				"as":           "lastMatchedOrder",
				"localField":   "lastMatchedOrderRef.id",
				"foreignField": "_id",
			},
		},
	}

	pipeline = append(pipeline, bson.M{
		// Get the preceding scheduled order
		"$lookup": bson.M{
			"from":         params.targetCollection,
			"as":           "precedingDocument",
			"localField":   "lastMatchedOrderRef.id",
			"foreignField": "lastMatchedOrder._id",
			"pipeline": []bson.M{
				{
					"$sort": bson.M{
						"createdTime": -1,
					},
				},
				{
					"$limit": 1,
				},
				{
					"$project": bson.M{
						"_id":         1,
						"createdTime": 1,
					},
				},
			},
		},
	})

	pipeline = append(pipeline,
		bson.M{
			"$replaceRoot": bson.M{
				"newRoot": bson.M{
					"userId":      "$userId",
					"clinicId":    "$clinicId",
					"createdTime": now,
					"lastMatchedOrder": bson.M{
						"$arrayElemAt": bson.A{"$lastMatchedOrder", 0},
					},
					"precedingDocument": bson.M{
						"$arrayElemAt": bson.A{"$precedingDocument", 0},
					},
				},
			},
		},
		bson.M{
			"$merge": bson.M{
				"into": params.targetCollection,
			},
		},
	)

	return pipeline
}
