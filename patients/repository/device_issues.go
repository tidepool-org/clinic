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
// Criteria so far, in order of precedence:
//   - Expired provider-specific invitation: the newest connection request for the primary
//     issue's provider has expired, and no data source for that provider was created
//     since. The issue is classified as expiredInvite, effective at the request's
//     expiration time.
//   - Stale invitation: the newest connection request for the primary issue's provider
//     has gone unaccepted for PendingDataSourceStaleDuration, that is, no data source for
//     that provider was created after it. The issue is classified as staleInvite,
//     effective when the request went stale.
//   - Stale data: the connected data source for the primary issue's provider has had no
//     new data for DataSourceStaleDataDuration, and no connection request for that
//     provider is newer than the data source. The issue is classified as staleData,
//     effective when the data went stale.
//   - Stale device-non-specific invitation: the patient's outstanding issue is the
//     invitation to claim the account, the patient was created more than
//     PendingDataSourceStaleDuration ago, and has no connection requests and no data
//     sources. The issue is classified as staleInvite, effective when the invitation went
//     stale.
func (r *repository) UpdateDeviceIssues(ctx context.Context) error {
	// Provider-specific issues are checked against their connection requests and data
	// sources; the device-non-specific invitation against the patient's age.
	sources := append(
		append([]string{}, patients.DataSourceProviderNames...),
		patients.PrimaryIssueSourceDeviceNonSpecificInvite,
	)
	selector := bson.M{"primaryIssue.source": bson.M{"$in": sources}}

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
				"case": expiredInvite(now),
				"then": bson.M{
					"kind":          patients.PrimaryIssueKindExpiredInvite,
					"effectiveTime": "$$request.expirationTime",
				},
			},
			bson.M{
				"case": staleInvite(now),
				"then": bson.M{
					"kind":          patients.PrimaryIssueKindStaleInvite,
					"effectiveTime": requestStaleAt(),
				},
			},
			bson.M{
				"case": dataStale(now),
				"then": bson.M{
					"kind":          patients.PrimaryIssueKindStaleData,
					"effectiveTime": dataStaleAt(),
				},
			},
			bson.M{
				"case": nonSpecificInviteStale(now),
				"then": bson.M{
					"kind":          patients.PrimaryIssueKindStaleInvite,
					"effectiveTime": patientStaleAt(),
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
			"request":             newestRequestForSource(),
			"dataSourceCreated":   newestDataSourceCreatedForSource(),
			"connectedDataSource": newestConnectedDataSourceForSource(),
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

// expiredInvite builds the expression that checks for expired invitations. It is true
// when the newest connection request for the primary issue's provider ($$request) has an
// expiration time that has passed, and the newest data source for that provider
// ($$dataSourceCreated, its creation time) is absent or predates the request. A request
// without an expiration time never expires.
func expiredInvite(now time.Time) bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{bson.M{"$type": "$$request.expirationTime"}, "date"}},
		bson.M{"$lt": bson.A{"$$request.expirationTime", now}},
		bson.M{"$or": bson.A{
			bson.M{"$eq": bson.A{"$$dataSourceCreated", nil}},
			bson.M{"$lt": bson.A{"$$dataSourceCreated", "$$request.createdTime"}},
		}},
	}}
}

// staleInvite builds the expression for the second criterion. It is true when the
// newest connection request for the primary issue's provider ($$request) went stale
// before now without being accepted, meaning no data source for that provider
// ($$dataSourceCreated, the newest one's creation time) was created after the request.
// Expiry is checked first, so this applies to requests aged between
// PendingDataSourceStaleDuration and PendingDataSourceExpirationDuration.
func staleInvite(now time.Time) bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{bson.M{"$type": "$$request.createdTime"}, "date"}},
		bson.M{"$lte": bson.A{requestStaleAt(), now}},
		bson.M{"$or": bson.A{
			bson.M{"$eq": bson.A{"$$dataSourceCreated", nil}},
			bson.M{"$lte": bson.A{"$$dataSourceCreated", "$$request.createdTime"}},
		}},
	}}
}

// requestStaleAt builds the expression for the time the newest connection request
// ($$request) goes stale: PendingDataSourceStaleDuration after its creation. Adding a
// number of milliseconds to a date yields a date.
func requestStaleAt() bson.M {
	return bson.M{"$add": bson.A{
		"$$request.createdTime",
		patients.PendingDataSourceStaleDuration.Milliseconds(),
	}}
}

// dataStale builds the expression for the third criterion. It is true when the connected
// data source for the primary issue's provider ($$connectedDataSource) last received data
// more than DataSourceStaleDataDuration ago, and no connection request for that provider
// ($$request being the newest) is newer than the data source. A data source without a
// latestDataTime, or no connected data source at all, never qualifies.
func dataStale(now time.Time) bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{
			bson.M{"$type": "$$connectedDataSource.latestDataTime"}, "date",
		}},
		bson.M{"$lte": bson.A{dataStaleAt(), now}},
		bson.M{"$or": bson.A{
			bson.M{"$eq": bson.A{"$$request", nil}},
			bson.M{"$lte": bson.A{
				"$$request.createdTime", "$$connectedDataSource.createdTime",
			}},
		}},
	}}
}

// dataStaleAt builds the expression for the time the connected data source's data
// ($$connectedDataSource) goes stale: DataSourceStaleDataDuration after its latest data.
func dataStaleAt() bson.M {
	return bson.M{"$add": bson.A{
		"$$connectedDataSource.latestDataTime",
		patients.DataSourceStaleDataDuration.Milliseconds(),
	}}
}

// nonSpecificInviteStale builds the expression for the fourth criterion. It is true when
// the primary issue is the device-non-specific invitation to claim the account, the patient
// was created more than PendingDataSourceStaleDuration ago, and the patient has no
// connection requests for any provider and no data sources. Requests are counted, not
// provider keys, so a provider entry holding an empty array counts as none.
func nonSpecificInviteStale(now time.Time) bson.M {
	requestCount := bson.M{"$sum": bson.M{"$map": bson.M{
		"input": bson.M{"$objectToArray": bson.M{
			"$ifNull": bson.A{"$providerConnectionRequests", bson.M{}},
		}},
		"in": bson.M{"$size": "$$this.v"},
	}}}
	dataSourceCount := bson.M{"$size": bson.M{"$ifNull": bson.A{"$dataSources", bson.A{}}}}
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{
			"$primaryIssue.source", patients.PrimaryIssueSourceDeviceNonSpecificInvite,
		}},
		bson.M{"$eq": bson.A{bson.M{"$type": "$createdTime"}, "date"}},
		bson.M{"$lte": bson.A{patientStaleAt(), now}},
		bson.M{"$eq": bson.A{requestCount, 0}},
		bson.M{"$eq": bson.A{dataSourceCount, 0}},
	}}
}

// patientStaleAt builds the expression for the time the patient's invitation to claim
// the account goes stale: PendingDataSourceStaleDuration after the patient was created.
func patientStaleAt() bson.M {
	return bson.M{"$add": bson.A{
		"$createdTime",
		patients.PendingDataSourceStaleDuration.Milliseconds(),
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
	// $first of an empty array yields a missing value, which a variable does not equate to
	// null, so normalise it.
	return bson.M{"$ifNull": bson.A{bson.M{"$first": requests}, nil}}
}

// newestConnectedDataSourceForSource builds the expression for the connected data source
// whose provider is the primary issue's source, or null when there is none. Should several
// be connected, the most recently created one is chosen.
func newestConnectedDataSourceForSource() bson.M {
	matching := bson.M{"$filter": bson.M{
		"input": bson.M{"$ifNull": bson.A{"$dataSources", bson.A{}}},
		"cond": bson.M{"$and": bson.A{
			bson.M{"$eq": bson.A{"$$this.providerName", "$primaryIssue.source"}},
			bson.M{"$eq": bson.A{"$$this.state", patients.DataSourceStateConnected}},
		}},
	}}
	newest := bson.M{"$first": bson.M{"$sortArray": bson.M{
		"input":  matching,
		"sortBy": bson.M{"createdTime": -1},
	}}}
	// As in newestRequestForSource, normalise a missing value to null.
	return bson.M{"$ifNull": bson.A{newest, nil}}
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
