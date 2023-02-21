package summary

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"regexp"
)

const (
	summaryCollectionName = "summary"
)

// go:generate mockgen --build_flags=--mod=mod -source=./repo.go -destination=./test/mock_repository.go -package test -aux_files=github.com/tidepool-org/clinic/summary=summary.go MockRepository

var collation = options.Collation{Locale: "en", Strength: 1}

type Repository[T Period] interface {
	Service[T]
}

func NewRepository[T Period](db *mongo.Database, logger *zap.SugaredLogger, lifecycle fx.Lifecycle) (Repository[T], error) {
	repo := &repository[T]{
		collection: db.Collection(summaryCollectionName),
		logger:     logger,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return repo.Initialize(ctx)
		},
	})

	return repo, nil
}

type repository[T Period] struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func (r *repository[T]) Initialize(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "userId", Value: 1},
				{Key: "period", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("UniqueUserSummaryType"),
		},

		{
			Keys: bson.D{
				{Key: "stats.clinicId", Value: 1},
				{Key: "dates.lastUploadDate", Value: 1},
			},
			Options: options.Index().
				SetName("LastUploadDate"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeCGMUsePercent", Value: 1},
				{Key: "stats.hasTimeCGMUsePercent", Value: -1},
			},
			Options: options.Index().
				SetName("CGMUsePercent"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.glucoseManagementIndicator", Value: 1},
				{Key: "stats.hasGlucoseManagementIndicator", Value: -1},
			},
			Options: options.Index().
				SetName("GlucoseManagementIndicator"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.averageGlucose", Value: 1},
				{Key: "stats.hasAverageGlucose", Value: -1},
			},
			Options: options.Index().
				SetName("GlucoseManagementIndicator"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeInVeryLowPercent", Value: 1},
				{Key: "stats.hasTimeInVeryLowPercent", Value: -1},
			},
			Options: options.Index().
				SetName("TimeInVeryLowPercent"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeInLowPercent", Value: 1},
				{Key: "stats.hasTimeInLowPercent", Value: -1},
			},
			Options: options.Index().
				SetName("TimeInLowPercent"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeInTargetPercent", Value: 1},
				{Key: "stats.hasTimeInTargetPercent", Value: -1},
			},
			Options: options.Index().
				SetName("TimeInTargetPercent"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeInHighPercent", Value: 1},
				{Key: "stats.hasTimeInHighPercent", Value: -1},
			},
			Options: options.Index().
				SetName("TimeInHighPercent"),
		},

		{
			Keys: bson.D{
				{Key: "patients.clinicId", Value: 1},
				{Key: "stats.timeInVeryHighPercent", Value: 1},
				{Key: "stats.hasTimeInVeryHighPercent", Value: -1},
			},
			Options: options.Index().
				SetName("TimeInVeryHighPercent"),
		},
	})
	return err
}

func (r *repository[T]) Get(ctx context.Context, userId string) (*Summary[T], error) {
	summary := &Summary[T]{}

	selector := bson.M{
		"type":   summary.Stats.GetType(),
		"userId": userId,
	}

	err := r.collection.FindOne(ctx, selector).Decode(&summary)
	if err == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	return summary, nil
}

func (r *repository[T]) Remove(ctx context.Context, userId string) error {
	selector := bson.M{
		"type":   GetTypeString[T](),
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

func (r *repository[T]) List(ctx context.Context, filter *Filter, pagination store.Pagination, sorts []*store.Sort) (*ListResult[T], error) {
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

	var result ListResult[T]
	if err = cursor.Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding patients list: %w", err)
	}

	if result.TotalCount == 0 {
		result.Patients = make([]*Summary[T], 0)
	}

	return &result, nil
}

func (r *repository[T]) CreateOrUpdate(ctx context.Context, summary *Summary[T]) error {
	if ctx == nil {
		return errors.New("context is missing")
	}
	if summary == nil {
		return errors.New("summary object is missing")
	}

	expectedType := GetTypeString[T]()
	if summary.Type != expectedType {
		return fmt.Errorf("invalid summary type %v, expected %v", summary.Type, expectedType)
	}

	if summary.UserID == "" {
		return errors.New("summary missing UserID")
	}

	opts := options.Update().SetUpsert(true)
	selector := bson.M{
		"userId": summary.UserID,
		"type":   summary.Type,
	}

	res, err := r.collection.UpdateOne(ctx, selector, bson.M{"$set": summary}, opts)
	if err != nil {
		return fmt.Errorf("error updating patient: %w", err)
	} else if res.ModifiedCount == 0 {
		return ErrNotFound
	}

	return nil
}

func generateListFilterQuery(filter *Filter) bson.M {
	// TODO this needs help, search needs to be appended to the elemmatch if it exists, else make new
	selector := bson.M{}
	if filter.ClinicId != nil {
		clinicId := *filter.ClinicId
		clinicObjId, _ := primitive.ObjectIDFromHex(clinicId)
		selector["patients"] = bson.M{
			"$elemMatch": bson.M{"clinicId": clinicObjId},
		}
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
		selector["dates.lastUploadDate"] = lastUploadDate
	}

	MaybeApplyNumericFilter(selector,
		"stats.timeCGMUsePercent",
		filter.TimeCGMUsePercentCmp,
		filter.TimeCGMUsePercentValue,
	)

	MaybeApplyNumericFilter(selector,
		"stats.timeInVeryLowPercent",
		filter.TimeInVeryLowPercentCmp,
		filter.TimeInVeryLowPercentValue,
	)

	MaybeApplyNumericFilter(selector,
		"stats.timeInLowPercent",
		filter.TimeInLowPercentCmp,
		filter.TimeInLowPercentValue,
	)

	MaybeApplyNumericFilter(selector,
		"stats.timeInTargetPercent",
		filter.TimeInTargetPercentCmp,
		filter.TimeInTargetPercentValue,
	)

	MaybeApplyNumericFilter(selector,
		"stats.timeInHighPercent",
		filter.TimeInHighPercentCmp,
		filter.TimeInHighPercentValue,
	)

	MaybeApplyNumericFilter(selector,
		"stats.timeInVeryHighPercent",
		filter.TimeInVeryHighPercentCmp,
		filter.TimeInVeryHighPercentValue,
	)

	return selector
}

func MaybeApplyNumericFilter(selector bson.M, field string, cmp *string, value float64) {
	if operator, ok := cmpToMongoFilter(cmp); ok {
		// ugly, but needed to ensure index prefix
		for _, filterable := range filterablePeriodFields {
			if _, exists := selector["stats."+filterable]; !exists {
				selector["stats."+filterable] = bson.M{"$ne": -1111}
			}
		}

		selector["stats."+field] = bson.M{operator: value}
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
	"patients.fullName":       {},
	"patients.birthDate":      {},
	"dates.lastUploadDate":    {},
	"dates.hasLastUploadDate": {},

	"stats.timeCGMUsePercent":             {},
	"stats.hasTimeCGMUsePercent":          {},
	"stats.glucoseManagementIndicator":    {},
	"stats.hasGlucoseManagementIndicator": {},
	"stats.hasAverageGlucose":             {},
	"stats.hasTimeInLowPercent":           {},
	"stats.hasTimeInVeryLowPercent":       {},
	"stats.hasTimeInHighPercent":          {},
	"stats.hasTimeInVeryHighPercent":      {},
	"stats.hasTimeInTargetPercent":        {},
}

var filterablePeriodFields = []string{
	"timeCGMUsePercent",
	"timeInVeryHighPercent",
	"timeInHighPercent",
	"timeInTargetPercent",
	"timeInLowPercent",
	"timeInVeryLowPercent",
}
