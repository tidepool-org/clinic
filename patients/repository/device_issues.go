package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/tidepool-org/clinic/patients"
)

// deviceIssueField is a temporary field holding the outcome of the device issue criteria
// while the update pipeline runs. It never reaches the stored document.
const deviceIssueField = "deviceIssue"

// UpdateDeviceIssues re-evaluates the primary issue of every patient against the device
// issue criteria, in one update across all patients. Each patient is examined and written
// at most once, and a patient whose primary issue already reflects the outcome is left
// untouched, so repeated runs don't bump updatedTime.
//
// Criteria so far:
//   - Expired provider-specific invitation: the newest connection request for the primary
//     issue's provider has expired, and no data source for that provider was created
//     since. The issue is classified as invitationExpired, effective at the request's
//     expiration time.
func (r *repository) UpdateDeviceIssues(ctx context.Context) error {
	// Only provider-specific issues have connection requests or data sources to check.
	selector := bson.M{
		"primaryIssue.source": bson.M{"$in": patients.DataSourceProviderNames},
	}

	noOutcome := bson.M{"$eq": bson.A{"$" + deviceIssueField, nil}}
	update := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			deviceIssueField: deviceIssueOutcome(time.Now()),
		}}},
		bson.D{{Key: "$set", Value: bson.M{
			"primaryIssue": bson.M{"$cond": bson.A{
				noOutcome,
				"$primaryIssue",
				bson.M{"$mergeObjects": bson.A{"$primaryIssue", "$" + deviceIssueField}},
			}},
			"updatedTime": bson.M{"$cond": bson.A{noOutcome, "$updatedTime", "$$NOW"}},
		}}},
		bson.D{{Key: "$unset", Value: deviceIssueField}},
	}

	if _, err := r.collection.UpdateMany(ctx, selector, update); err != nil {
		r.logger.Errorw("error updating patient device issues", "error", err)
		return fmt.Errorf("error updating patient device issues: %w", err)
	}
	return nil
}

// deviceIssueOutcome builds the expression that evaluates the device issue criteria for
// one patient. It yields the fields to merge into the primary issue, or null when no
// criterion applies or the primary issue already carries the outcome. Each criterion is
// a $switch branch; the first that applies wins.
func deviceIssueOutcome(now time.Time) bson.M {
	criteria := bson.M{"$switch": bson.M{
		"branches": bson.A{
			bson.M{
				"case": invitationExpired(now),
				"then": bson.M{
					"kind":          patients.PrimaryIssueKindInvitationExpired,
					"effectiveTime": "$$request.expirationTime",
				},
			},
		},
		"default": nil,
	}}
	alreadyApplied := bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{"$$outcome.kind", "$primaryIssue.kind"}},
		bson.M{"$eq": bson.A{"$$outcome.effectiveTime", "$primaryIssue.effectiveTime"}},
	}}
	return bson.M{"$let": bson.M{
		"vars": bson.M{
			"request":           newestRequestForSource(),
			"dataSourceCreated": newestDataSourceCreatedForSource(),
		},
		"in": bson.M{"$let": bson.M{
			"vars": bson.M{"outcome": criteria},
			"in": bson.M{"$cond": bson.A{
				bson.M{"$or": bson.A{
					bson.M{"$eq": bson.A{"$$outcome", nil}},
					alreadyApplied,
				}},
				nil,
				"$$outcome",
			}},
		}},
	}}
}

// invitationExpired builds the expression that checks for expired invitations. It is true
// when the newest connection request for the primary issue's provider ($$request) has an
// expiration time that has passed, and the newest data source for that provider
// ($$dataSourceCreated, its creation time) is absent or predates the request. A request
// without an expiration time never expires.
func invitationExpired(now time.Time) bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{bson.M{"$type": "$$request.expirationTime"}, "date"}},
		bson.M{"$lt": bson.A{"$$request.expirationTime", now}},
		bson.M{"$or": bson.A{
			bson.M{"$eq": bson.A{"$$dataSourceCreated", nil}},
			bson.M{"$lt": bson.A{"$$dataSourceCreated", "$$request.createdTime"}},
		}},
	}}
}

// newestRequestForSource builds the expression for the newest connection request whose
// provider is the primary issue's source, or null when there is none. Requests are stored
// per provider, newest first. The provider is only known per document, so the map is
// searched with $objectToArray rather than addressed by a computed field name, which
// keeps the pipeline valid on MongoDB 6.0.
func newestRequestForSource() bson.M {
	requestsBySource := bson.M{"$filter": bson.M{
		"input": bson.M{"$objectToArray": bson.M{
			"$ifNull": bson.A{"$providerConnectionRequests", bson.M{}},
		}},
		"cond": bson.M{"$eq": bson.A{"$$this.k", "$primaryIssue.source"}},
	}}
	requests := bson.M{"$ifNull": bson.A{
		bson.M{"$first": bson.M{"$map": bson.M{
			"input": requestsBySource,
			"in":    "$$this.v",
		}}},
		bson.A{},
	}}
	return bson.M{"$first": requests}
}

// newestDataSourceCreatedForSource builds the expression for the creation time of the
// newest data source whose provider is the primary issue's source, or null when there is
// none. Data sources without a creation time are ignored, as they can't be shown to be
// newer than anything.
func newestDataSourceCreatedForSource() bson.M {
	matching := bson.M{"$filter": bson.M{
		"input": bson.M{"$ifNull": bson.A{"$dataSources", bson.A{}}},
		"cond":  bson.M{"$eq": bson.A{"$$this.providerName", "$primaryIssue.source"}},
	}}
	return bson.M{"$max": bson.M{"$map": bson.M{
		"input": matching,
		"in":    "$$this.createdTime",
	}}}
}
