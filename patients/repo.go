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
	"go.uber.org/zap"
	"regexp"
	"time"
)

const (
	patientsCollectionName = "patients"
)

// Collation to use for string fields
var collation = options.Collation{Locale: "en", Strength: 1}

//go:generate mockgen --build_flags=--mod=mod -source=./repo.go -destination=./test/mock_repository.go -package test -aux_files=github.com/tidepool-org/clinic/patients=patients.go MockRepository

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
				SetBackground(true).
				SetUnique(true).
				SetName("UniquePatient"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "fullName", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientFullNameEn").
				SetCollation(&collation),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "birthDate", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientBirthDate"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "email", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientEmail"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "mrn", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientMRN"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.lastUploadDate", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryLastUploadDate"),
		},

		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeCGMUsePercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeCGMUsePercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeCGMUse1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.glucoseManagementIndicator", Value: 1},
				{Key: "summary.periods.1d.hasGlucoseManagementIndicator", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryGMI1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeInVeryLowPercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeInVeryLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryLow1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeInLowPercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeInLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInLow1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeInTargetPercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeInTargetPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInTarget1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeInHighPercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeInHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInHigh1d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.1d.timeInVeryHighPercent", Value: 1},
				{Key: "summary.periods.1d.hasTimeInVeryHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryHigh1d"),
		},

		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeCGMUsePercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeCGMUsePercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeCGMUse7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.glucoseManagementIndicator", Value: 1},
				{Key: "summary.periods.7d.hasGlucoseManagementIndicator", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryGMI7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeInVeryLowPercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeInVeryLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryLow7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeInLowPercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeInLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInLow7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeInTargetPercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeInTargetPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInTarget7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeInHighPercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeInHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInHigh7d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.7d.timeInVeryHighPercent", Value: 1},
				{Key: "summary.periods.7d.hasTimeInVeryHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryHigh7d"),
		},

		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeCGMUsePercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeCGMUsePercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeCGMUse14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.glucoseManagementIndicator", Value: 1},
				{Key: "summary.periods.14d.hasGlucoseManagementIndicator", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryGMI14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeInVeryLowPercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeInVeryLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryLow14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeInLowPercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeInLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInLow14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeInTargetPercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeInTargetPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInTarget14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeInHighPercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeInHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInHigh14d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.14d.timeInVeryHighPercent", Value: 1},
				{Key: "summary.periods.14d.hasTimeInVeryHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryHigh14d"),
		},

		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeCGMUsePercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeCGMUsePercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeCGMUse30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.glucoseManagementIndicator", Value: 1},
				{Key: "summary.periods.30d.hasGlucoseManagementIndicator", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryGMI30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeInVeryLowPercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeInVeryLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryLow30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeInLowPercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeInLowPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInLow30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeInTargetPercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeInTargetPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInTarget30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeInHighPercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeInHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInHigh30d"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.periods.30d.timeInVeryHighPercent", Value: 1},
				{Key: "summary.periods.30d.hasTimeInVeryHighPercent", Value: -1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryHigh30d"),
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

	var hasFullNameSort = false
	for _, sort := range sorts {
		if sort.Attribute == "fullName" {
			hasFullNameSort = true
		}
	}

	var opts *options.AggregateOptions
	if len(sorts) == 0 || hasFullNameSort {
		// Case insensitive sorting when sorting by fullName
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

	var result ListResult
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

	update := bson.M{}
	if summary == nil {
		update["$unset"] = bson.M{
			"summary":     "",
			"updatedTime": time.Now(),
		}
	} else {
		update["$set"] = bson.M{
			"summary":     summary,
			"updatedTime": time.Now(),
		}
	}

	res, err := r.collection.UpdateMany(ctx, selector, update)
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
	lastUploadDate := bson.M{}
	if filter.LastUploadDateFrom != nil && !filter.LastUploadDateFrom.IsZero() {
		lastUploadDate["$gte"] = filter.LastUploadDateFrom
	}
	if filter.LastUploadDateTo != nil && !filter.LastUploadDateTo.IsZero() {
		lastUploadDate["$lt"] = filter.LastUploadDateTo
	}
	if len(lastUploadDate) > 0 {
		selector["summary.lastUploadDate"] = lastUploadDate
	}

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeCGMUsePercent",
		filter.TimeCGMUsePercentCmp1d,
		filter.TimeCGMUsePercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeInVeryLowPercent",
		filter.TimeInVeryLowPercentCmp1d,
		filter.TimeInVeryLowPercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeInLowPercent",
		filter.TimeInLowPercentCmp1d,
		filter.TimeInLowPercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeInTargetPercent",
		filter.TimeInTargetPercentCmp1d,
		filter.TimeInTargetPercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeInHighPercent",
		filter.TimeInHighPercentCmp1d,
		filter.TimeInHighPercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.1d.timeInVeryHighPercent",
		filter.TimeInVeryHighPercentCmp1d,
		filter.TimeInVeryHighPercentValue1d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeCGMUsePercent",
		filter.TimeCGMUsePercentCmp7d,
		filter.TimeCGMUsePercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.7d.timeInVeryLowPercent",
		filter.TimeInVeryLowPercentCmp7d,
		filter.TimeInVeryLowPercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.7d.timeInLowPercent",
		filter.TimeInLowPercentCmp7d,
		filter.TimeInLowPercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.7d.timeInTargetPercent",
		filter.TimeInTargetPercentCmp7d,
		filter.TimeInTargetPercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.7d.timeInHighPercent",
		filter.TimeInHighPercentCmp7d,
		filter.TimeInHighPercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.7d.timeInVeryHighPercent",
		filter.TimeInVeryHighPercentCmp7d,
		filter.TimeInVeryHighPercentValue7d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeCGMUsePercent",
		filter.TimeCGMUsePercentCmp14d,
		filter.TimeCGMUsePercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeInVeryLowPercent",
		filter.TimeInVeryLowPercentCmp14d,
		filter.TimeInVeryLowPercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeInLowPercent",
		filter.TimeInLowPercentCmp14d,
		filter.TimeInLowPercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeInTargetPercent",
		filter.TimeInTargetPercentCmp14d,
		filter.TimeInTargetPercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeInHighPercent",
		filter.TimeInHighPercentCmp14d,
		filter.TimeInHighPercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.14d.timeInVeryHighPercent",
		filter.TimeInVeryHighPercentCmp14d,
		filter.TimeInVeryHighPercentValue14d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeCGMUsePercent",
		filter.TimeCGMUsePercentCmp30d,
		filter.TimeCGMUsePercentValue30d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeInVeryLowPercent",
		filter.TimeInVeryLowPercentCmp30d,
		filter.TimeInVeryLowPercentValue30d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeInLowPercent",
		filter.TimeInLowPercentCmp30d,
		filter.TimeInLowPercentValue30d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeInTargetPercent",
		filter.TimeInTargetPercentCmp30d,
		filter.TimeInTargetPercentValue30d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeInHighPercent",
		filter.TimeInHighPercentCmp30d,
		filter.TimeInHighPercentValue30d,
	)

	MaybeApplyNumericFilter(selector,
		"summary.periods.30d.timeInVeryHighPercent",
		filter.TimeInVeryHighPercentCmp30d,
		filter.TimeInVeryHighPercentValue30d,
	)

	return selector
}

func MaybeApplyNumericFilter(selector bson.M, field string, cmp *string, value float64) {
	if f, ok := cmpToMongoFilter(cmp); ok {
		selector[field] = bson.M{f: value}
	}
}

func isSortAttributeValid(attribute string) bool {
	_, ok := validSortAttributes[attribute]
	return ok
}

func generateListSortStage(sorts []*store.Sort) bson.D {
	var s bson.D
	for _, sort := range sorts {
		if sort != nil && isSortAttributeValid(sort.Attribute) {
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

var validSortAttributes = map[string]struct{}{
	"fullName":                  {},
	"birthDate":                 {},
	"summary.lastUploadDate":    {},
	"summary.hasLastUploadDate": {},

	"summary.periods.1d.timeCGMUsePercent":             {},
	"summary.periods.1d.hasTimeCGMUsePercent":          {},
	"summary.periods.1d.glucoseManagementIndicator":    {},
	"summary.periods.1d.hasGlucoseManagementIndicator": {},
	"summary.periods.1d.hasAverageGlucose":             {},
	"summary.periods.1d.hasTimeInLowPercent":           {},
	"summary.periods.1d.hasTimeInVeryLowPercent":       {},
	"summary.periods.1d.hasTimeInHighPercent":          {},
	"summary.periods.1d.hasTimeInVeryHighPercent":      {},
	"summary.periods.1d.hasTimeInTargetPercent":        {},

	"summary.periods.7d.timeCGMUsePercent":             {},
	"summary.periods.7d.hasTimeCGMUsePercent":          {},
	"summary.periods.7d.glucoseManagementIndicator":    {},
	"summary.periods.7d.hasGlucoseManagementIndicator": {},
	"summary.periods.7d.hasAverageGlucose":             {},
	"summary.periods.7d.hasTimeInLowPercent":           {},
	"summary.periods.7d.hasTimeInVeryLowPercent":       {},
	"summary.periods.7d.hasTimeInHighPercent":          {},
	"summary.periods.7d.hasTimeInVeryHighPercent":      {},
	"summary.periods.7d.hasTimeInTargetPercent":        {},

	"summary.periods.14d.timeCGMUsePercent":             {},
	"summary.periods.14d.hasTimeCGMUsePercent":          {},
	"summary.periods.14d.glucoseManagementIndicator":    {},
	"summary.periods.14d.hasGlucoseManagementIndicator": {},
	"summary.periods.14d.hasAverageGlucose":             {},
	"summary.periods.14d.hasTimeInLowPercent":           {},
	"summary.periods.14d.hasTimeInVeryLowPercent":       {},
	"summary.periods.14d.hasTimeInHighPercent":          {},
	"summary.periods.14d.hasTimeInVeryHighPercent":      {},
	"summary.periods.14d.hasTimeInTargetPercent":        {},

	"summary.periods.30d.timeCGMUsePercent":             {},
	"summary.periods.30d.hasTimeCGMUsePercent":          {},
	"summary.periods.30d.glucoseManagementIndicator":    {},
	"summary.periods.30d.hasGlucoseManagementIndicator": {},
	"summary.periods.30d.hasAverageGlucose":             {},
	"summary.periods.30d.hasTimeInLowPercent":           {},
	"summary.periods.30d.hasTimeInVeryLowPercent":       {},
	"summary.periods.30d.hasTimeInHighPercent":          {},
	"summary.periods.30d.hasTimeInVeryHighPercent":      {},
	"summary.periods.30d.hasTimeInTargetPercent":        {},
}
