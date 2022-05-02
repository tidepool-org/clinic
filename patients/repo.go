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
				SetName("PatientFullName"),
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
				{Key: "fullName", Value: "text"},
				{Key: "email", Value: "text"},
				{Key: "mrn", Value: "text"},
				{Key: "birthDate", Value: "text"},
				{Key: "firstName", Value: "text"},
				{Key: "lastName", Value: "text"},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSearch").
				SetWeights(bson.M{
					"fullName":  3,
					"email":     2,
					"mrn":       1,
					"birthDate": 1,
				}),
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
				{Key: "summary.timeCGMUse", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeCGMUse"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.glucoseManagementIndicator", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryGMI"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.percentTimeInVeryLow", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryLow"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.percentTimeInLow", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInLow"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.percentTimeInTarget", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInTarget"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.percentTimeInHigh", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInHigh"),
		},
		{
			Keys: bson.D{
				{Key: "clinicId", Value: 1},
				{Key: "summary.percentTimeInVeryHigh", Value: 1},
			},
			Options: options.Index().
				SetBackground(true).
				SetName("PatientSummaryTimeInVeryHigh"),
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

func (r *repository) List(ctx context.Context, filter *Filter, pagination store.Pagination, sort *store.Sort) (*ListResult, error) {
	// We use an aggregation pipeline with facet in order to get the count
	// and the patients from a single query
	pipeline := []bson.M{
		{"$match": generateListFilterQuery(filter)},
		{"$sort": generateListSortStage(sort)},
	}
	pipeline = append(pipeline, generatePaginationFacetStages(pagination)...)

	r.logger.Debugw("retrieving list of patients", "pipeline", pipeline)

	cursor, err := r.collection.Aggregate(ctx, pipeline)
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

	err := r.collection.FindOneAndUpdate(ctx, selector, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrNotFound
		}
		return fmt.Errorf("error updating patient: %w", err)
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
	if filter.LastUploadDateFrom != nil && !filter.LastUploadDateFrom.IsZero() {
		selector["summary.lastUploadDate"] = bson.M{"$gte": filter.LastUploadDateFrom}
	}
	if filter.LastUploadDateTo != nil && !filter.LastUploadDateTo.IsZero() {
		selector["summary.lastUploadDate"] = bson.M{"$lt": filter.LastUploadDateTo}
	}

	MaybeApplyRangeFilter(selector,
		"summary.percentTimeInVeryLow",
		filter.PercentTimeInVeryLowCmp,
		filter.PercentTimeInVeryLowValue,
	)

	MaybeApplyRangeFilter(selector,
		"summary.percentTimeInLow",
		filter.PercentTimeInLowCmp,
		filter.PercentTimeInLowValue,
	)

	MaybeApplyRangeFilter(selector,
		"summary.percentTimeInTarget",
		filter.PercentTimeInTargetCmp,
		filter.PercentTimeInTargetValue,
	)

	MaybeApplyRangeFilter(selector,
		"summary.percentTimeInHigh",
		filter.PercentTimeInHighCmp,
		filter.PercentTimeInHighValue,
	)

	MaybeApplyRangeFilter(selector,
		"summary.percentTimeInVeryHigh",
		filter.PercentTimeInVeryHighCmp,
		filter.PercentTimeInVeryHighValue,
	)

	return selector
}

func MaybeApplyRangeFilter(selector bson.M, field string, cmp *string, value float64) {
	if f, ok := cmpToMongoFilter(cmp); ok {
		selector[field] = bson.M{f: value}
	}
}

func isSortAttributeValid(attribute string) bool {
	_, ok := validSortAttributes[attribute]
	return ok
}

func generateListSortStage(sort *store.Sort) bson.D {
	var s bson.D
	if sort != nil && isSortAttributeValid(sort.Attribute) {
		s = append(s, bson.E{Key: sort.Attribute, Value: sort.Order()})
	} else {
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
	"fullName":                           {},
	"birthDate":                          {},
	"summary.lastUploadDate":             {},
	"summary.timeCGMUse":                 {},
	"summary.glucoseManagementIndicator": {},
}
