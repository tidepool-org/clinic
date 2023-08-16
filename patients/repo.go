package patients

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	patientsCollectionName = "patients"
)

// Collation to use for string fields
var collation = options.Collation{Locale: "en", Strength: 1}

//go:generate mockgen --build_flags=--mod=mod -source=./repo.go -destination=./test/mock_repository.go -package test -aux_files=github.com/tidepool-org/clinic/patients=patients.go MockRepository

func ptr[T any](v T) *T {
	return &v
}

type Repository interface {
	Service
}

func NewRepository(db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository, error) {
	repo := &repository{
		collection: db.Collection(patientsCollectionName),
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
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func (r *repository) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
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

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination, sorts []*store.Sort) (*ListResult, error) {
	// We use an aggregation pipeline with facet in order to get the count
	// and the patients from a single query
	pipeline := []bson.M{
		{"$match": generateListFilterQuery(filter)},
		{"$sort": generateListSortStage(sorts)},
	}
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

	if result.TotalCount == 0 {
		result.Patients = make([]*Patient, 0)
	}

	return &result, nil
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
	if patients.TotalCount > 0 {
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

	return r.Get(ctx, patient.ClinicId.Hex(), *patient.UserId)
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
		if err == mongo.ErrNoDocuments {
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
		err = fmt.Errorf("partially updated %v out of %v clinician records: %w", result.ModifiedCount, result.MatchedCount, err)
	}
	if err != nil {
		r.logger.Errorw("error updating patient emails", "error", err, "userId", userId)
	}

	return err
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
		if err == mongo.ErrNoDocuments {
			return nil, ErrPermissionNotFound
		}
		return nil, fmt.Errorf("error removing permission: %w", err)
	}

	return r.Get(ctx, clinicId, userId)
}

func (r *repository) DeleteFromAllClinics(ctx context.Context, userId string) error {
	selector := bson.M{
		"userId": userId,
	}

	_, err := r.collection.DeleteMany(ctx, selector)
	return err
}

func (r *repository) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) error {
	clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
	selector := bson.M{
		"clinicId": clinicObjId,
		"$or": []bson.M{
			{"permissions.custodian": bson.M{"$exists": false}},
			{"permissions.custodian": bson.M{"$eq": false}},
		},
	}

	_, err := r.collection.DeleteMany(ctx, selector)
	return err
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

	res, err := r.collection.UpdateMany(ctx, selector, bson.M{"$set": set, "$unset": unset})
	if err != nil {
		return fmt.Errorf("error updating patient: %w", err)
	} else if res.ModifiedCount == 0 {
		return ErrNotFound
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
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, update.ClinicId, update.UserId)
}

func (r *repository) UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *LastRequestedDexcomConnectUpdate) (*Patient, error) {
	clinicObjId, _ := primitive.ObjectIDFromHex(update.ClinicId)
	currentTime := time.Now()

	// We fetch the current dexcom data source to determine if we are requesting an initial connection
	// or a reconnection to a previously connected data source, which will have a `ModifiedTime` set
	patient, err := r.Get(ctx, update.ClinicId, update.UserId)
	if err != nil {
		return nil, fmt.Errorf("error finding patient: %w", err)
	}

	var patientDexcomDataSource DataSource
	if patient.DataSources != nil {
		for _, source := range *patient.DataSources {
			if source.ProviderName == DexcomDataSourceProviderName {
				patientDexcomDataSource = source
			}
		}
	}

	selector := bson.M{
		"clinicId":                 clinicObjId,
		"userId":                   update.UserId,
		"dataSources.providerName": DexcomDataSourceProviderName,
	}

	// Default update for initial connection requests
	mongoUpdate := bson.M{
		"$set": bson.M{
			"lastRequestedDexcomConnectTime": update.Time,
			"updatedTime":                    currentTime,
			"dataSources.$.expirationTime":   currentTime.Add(PendingDexcomDataSourceExpirationDuration),
			"dataSources.$.state":            DataSourceStatePending,
		},
	}

	// Update for previously connected requests
	if patientDexcomDataSource.ModifiedTime != nil {
		mongoUpdate = bson.M{
			"$set": bson.M{
				"lastRequestedDexcomConnectTime": update.Time,
				"updatedTime":                    currentTime,
				"dataSources.$.expirationTime":   currentTime.Add(PendingDexcomDataSourceExpirationDuration),
				"dataSources.$.modifiedTime":     currentTime,
				"dataSources.$.state":            DataSourceStatePendingReconnect,
			},
		}
	}

	err = r.collection.FindOneAndUpdate(ctx, selector, mongoUpdate).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error updating patient: %w", err)
	}

	return r.Get(ctx, update.ClinicId, update.UserId)
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
		"tags":     bson.M{"$type": 10}, // BSON type 10 is `null`
		"userId":   bson.M{"$in": patientIds},
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
		"userId":   bson.M{"$in": patientIds},
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

func generateListFilterQuery(filter *Filter) bson.M {
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

	if f, ok := filter.CGMTime["lastUploadDate"]; ok {
		cgmLastUploadDate := bson.M{}

		if f.Min != nil {
			cgmLastUploadDate["$gte"] = f.Min
		}

		if f.Max != nil {
			cgmLastUploadDate["$lt"] = f.Max
		}

		selector["summary.cgmStats.dates.lastUploadDate"] = cgmLastUploadDate
	}

	if f, ok := filter.BGMTime["lastUploadDate"]; ok {
		bgmLastUploadDate := bson.M{}

		if f.Min != nil {
			bgmLastUploadDate["$gte"] = f.Min
		}

		if f.Max != nil {
			bgmLastUploadDate["$lt"] = f.Max
		}

		selector["summary.bgmStats.dates.lastUploadDate"] = bgmLastUploadDate
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
	for _, sort := range sorts {
		if sort != nil {
			s = append(s, bson.E{Key: sort.Attribute, Value: sort.Order()})
		}
	}

	if len(s) == 0 {
		s = append(s, bson.E{Key: "fullName", Value: 1})
	}

	// Including _id in the sort query ensures that $skip aggregation works correctly
	// See https://docs.mongodb.com/manual/reference/operator/aggregation/skip/
	// for more details
	s = append(s, bson.E{Key: "_id", Value: 1})

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

func (r *repository) TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error) {
	if params.Tags == nil || len(*params.Tags) < 1 {
		return nil, errors.New("no tags provided")
	}

	if params.CgmLastUploadDateFrom == nil {
		return nil, errors.New("no lastUploadDateFrom provided")
	}

	if params.CgmLastUploadDateTo == nil {
		return nil, errors.New("no lastUploadDateTo provided")
	}

	if params.CgmLastUploadDateFrom.After(*params.CgmLastUploadDateTo) || params.CgmLastUploadDateFrom.Equal(*params.CgmLastUploadDateTo) {
		return nil, errors.New("provided lastUploadDateFrom is after or equal to lastUploadDateTo")
	}

	if params.Period == nil {
		return nil, errors.New("no period provided")
	}

	if *params.Period != "1d" && *params.Period != "7d" && *params.Period != "14d" && *params.Period != "30d" {
		return nil, errors.New("provided period is not one of the valid periods")
	}

	categories := [...]map[string]string{
		{
			"heading": "timeInVeryLowPercent",
			"field":   "timeInVeryLowPercent",
			"comp":    "$gt",
			"val":     "0.01",
		},

		{
			"heading": "timeInLowPercent",
			"field":   "timeInLowPercent",
			"comp":    "$gt",
			"val":     "0.04",
		},

		{
			"heading": "dropInTimeInTargetPercent",
			"field":   "timeInTargetPercentDelta",
			"comp":    "$gt",
			"val":     "0.15",
		},

		{
			"heading": "timeInTargetPercent",
			"field":   "timeInTargetPercent",
			"comp":    "$lt",
			"val":     "0.7",
		},

		{
			"heading": "timeCGMUsePercent",
			"field":   "timeCGMUsePercent",
			"comp":    "$lt",
			"val":     "0.7",
		},
	}

	limit := 50
	exclusions := make([]*primitive.ObjectID, 0, 50)
	tide := Tide{
		Config: &TideConfig{
			ClinicId: &clinicId,
			Filters: &TideFilters{
				TimeInVeryLowPercent:      ptr(">0.01"),
				TimeInLowPercent:          ptr(">0.04"),
				DropInTimeInTargetPercent: ptr(">0.15"),
				TimeInTargetPercent:       ptr("<0.7"),
				TimeCGMUsePercent:         ptr("<0.7"),
			},
			HighGlucoseThreshold:     ptr(10.0),
			LastUploadDateFrom:       params.CgmLastUploadDateFrom,
			LastUploadDateTo:         params.CgmLastUploadDateTo,
			LowGlucoseThreshold:      ptr(3.9),
			Period:                   params.Period,
			SchemaVersion:            ptr(1),
			Tags:                     params.Tags,
			VeryHighGlucoseThreshold: ptr(13.9),
			VeryLowGlucoseThreshold:  ptr(3.0),
		},
		Results: &TideResults{},
	}

	for _, category := range categories {
		selector := bson.M{
			"clinicId": clinicId,
			"tags":     bson.M{"$in": params.Tags},
			"summary.cgmStats.dates.lastUploadDate": bson.A{
				bson.M{"$gt": params.CgmLastUploadDateFrom},
				bson.M{"$lte": params.CgmLastUploadDateTo},
			},
			"summary.cgmStats.periods." + *params.Period + "." + category["field"]: bson.M{category["comp"]: category["val"]},
			"_id": bson.M{"$nin": exclusions},
		}

		opts := options.Find()
		opts.SetLimit(int64(limit))

		if category["comp"] == "$gt" {
			opts.SetSort(bson.D{
				{"summary.cgmStats.periods." + *params.Period + "." + category["field"], -1},
			})
		} else {
			opts.SetSort(bson.D{
				{"summary.cgmStats.periods." + *params.Period + "." + category["field"], 1},
			})
		}

		cursor, err := r.collection.Find(ctx, selector, opts)
		if err != nil {
			return nil, err
		}

		var patients []*Patient
		err = cursor.Decode(patients)
		if err != nil {
			return nil, err
		}

		categoryResult := make([]TideResultPatient, 20)
		for _, patient := range patients {
			exclusions = append(exclusions, patient.Id)

			var tags []string
			for _, tag := range *patient.Tags {
				tags = append(tags, tag.String())
			}

			categoryResult = append(categoryResult, TideResultPatient{
				Patient: &TidePatient{
					Email:    patient.Email,
					FullName: patient.FullName,
					Id:       patient.UserId,
					Tags:     &tags,
				},
				AverageGlucoseMmol:         (*patient.Summary.CGM.Periods)[*params.Period].AverageGlucoseMmol,
				GlucoseManagementIndicator: (*patient.Summary.CGM.Periods)[*params.Period].GlucoseManagementIndicator,
				TimeCGMUseMinutes:          (*patient.Summary.CGM.Periods)[*params.Period].TimeCGMUseMinutes,
				TimeCGMUsePercent:          (*patient.Summary.CGM.Periods)[*params.Period].TimeCGMUsePercent,
				TimeInHighPercent:          (*patient.Summary.CGM.Periods)[*params.Period].TimeInHighPercent,
				TimeInLowPercent:           (*patient.Summary.CGM.Periods)[*params.Period].TimeInLowPercent,
				TimeInTargetPercent:        (*patient.Summary.CGM.Periods)[*params.Period].TimeInTargetPercent,
				TimeInTargetPercentDelta:   (*patient.Summary.CGM.Periods)[*params.Period].TimeInTargetPercentDelta,
				TimeInVeryHighPercent:      (*patient.Summary.CGM.Periods)[*params.Period].TimeInVeryHighPercent,
				TimeInVeryLowPercent:       (*patient.Summary.CGM.Periods)[*params.Period].TimeInVeryLowPercent,
			})
		}

		(*tide.Results)[category["heading"]] = &categoryResult

		limit -= len(patients)
		if limit < 1 {
			break
		}
	}

	if limit > 0 {
		selector := bson.M{
			"clinicId": clinicId,
			"tags":     bson.M{"$in": params.Tags},
			"summary.cgmStats.dates.lastUploadDate": bson.A{
				bson.M{"$gt": params.CgmLastUploadDateFrom},
				bson.M{"$lte": params.CgmLastUploadDateTo},
			},
			"_id": bson.M{"$nin": exclusions},
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

		var patients []*Patient
		err = cursor.Decode(patients)
		if err != nil {
			return nil, err
		}

	}

	return &tide, nil
}
