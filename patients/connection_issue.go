package patients

import "time"

type ConnectionIssueCause string

const (
	ConnectionIssueCauseError         ConnectionIssueCause = "error"
	ConnectionIssueCauseDisconnected  ConnectionIssueCause = "disconnected"
	ConnectionIssueCauseStaleData     ConnectionIssueCause = "staleData"
	ConnectionIssueCauseStaleInvite   ConnectionIssueCause = "staleInvite"
	ConnectionIssueCauseExpiredInvite ConnectionIssueCause = "expiredInvite"
)

// ConnectionIssue is the highest priority connection problem detected for a patient.
type ConnectionIssue struct {
	Cause ConnectionIssueCause `bson:"cause"`
	// Hidden is set by a clinician who does not want to see the issue. It is cleared
	// whenever the connection issue source or the cause changes.
	Hidden bool `bson:"hidden,omitempty"`
}

// Equal reports whether both issues are absent, or have the same cause. Hidden is
// deliberately ignored, so hiding an issue does not make it look changed.
func (c *ConnectionIssue) Equal(other *ConnectionIssue) bool {
	if c == nil || other == nil {
		return c == nil && other == nil
	}
	return c.Cause == other.Cause
}

// DetectConnectionIssue returns the patient's connection issue, or nil when there is
// none. Only patients whose connection issue source is a provider are considered. The
// conditions are checked in priority order and the first one that holds wins.
//
// The hidden flag of the stored issue is kept while the cause stays the same and dropped
// when the cause changes.
func (p Patient) DetectConnectionIssue(now time.Time) *ConnectionIssue {
	issue := p.detectConnectionIssue(now)
	if issue != nil && p.ConnectionIssue != nil && issue.Cause == p.ConnectionIssue.Cause {
		issue.Hidden = p.ConnectionIssue.Hidden
	}
	return issue
}

func (p Patient) detectConnectionIssue(now time.Time) *ConnectionIssue {
	provider := string(p.ConnectionIssueSource)
	if _, ok := ConnectionIssueSourceForProvider(provider); !ok {
		return nil
	}

	source := p.dataSourceFor(provider)
	request := p.pendingConnectionRequestFor(provider, source)

	switch {
	case source != nil && source.State == DataSourceStateError:
		return &ConnectionIssue{Cause: ConnectionIssueCauseError}
	case source != nil && source.State == DataSourceStateDisconnected:
		return &ConnectionIssue{Cause: ConnectionIssueCauseDisconnected}
	case source != nil && source.hasStaleData(now):
		return &ConnectionIssue{Cause: ConnectionIssueCauseStaleData}
	case request != nil && request.isExpired(now):
		return &ConnectionIssue{Cause: ConnectionIssueCauseExpiredInvite}
	case request != nil && request.isStale(now):
		return &ConnectionIssue{Cause: ConnectionIssueCauseStaleInvite}
	}

	return nil
}

func (p Patient) dataSourceFor(provider string) *DataSource {
	if p.DataSources == nil {
		return nil
	}
	var matching DataSources
	for _, ds := range *p.DataSources {
		if ds.ProviderName == provider {
			matching = append(matching, ds)
		}
	}
	return matching.Newest()
}

// pendingConnectionRequestFor returns the patient's most recent connection request of
// the provider, unless it has been accepted, in which case nil is returned. See
// DataSource.Accepted.
func (p Patient) pendingConnectionRequestFor(provider string,
	source *DataSource) *ConnectionRequest {

	var newest *ConnectionRequest
	for i, request := range p.ProviderConnectionRequests[provider] {
		if newest == nil || request.CreatedTime.After(newest.CreatedTime) {
			newest = &p.ProviderConnectionRequests[provider][i]
		}
	}
	if newest == nil {
		return nil
	}

	if source != nil && source.Accepted(*newest) {
		return nil
	}
	return newest
}

// Accepted reports whether the data source fulfilled the request. A source created after
// the request did. A source without a created time is a legacy record from before that
// field existed, so its modified time stands in. A source with neither time cannot be
// placed at all; its presence is the only evidence available, so it counts as accepted.
//
// Reconnecting reuses the existing source, so its created time predates a request that
// asked for the reconnection. Its connected time is set on reconnecting though, so a
// source connected after the request fulfilled it, even if it has since disconnected or
// errored. A source that is connected fulfills the request too, even when it has been
// connected since before the request: a healthy connection wins over an unanswered
// request.
func (ds DataSource) Accepted(request ConnectionRequest) bool {
	at := ds.CreatedTime
	if at == nil {
		at = ds.ModifiedTime
	}
	if at == nil || at.After(request.CreatedTime) {
		return true
	}
	if ds.State == DataSourceStateConnected {
		return true
	}
	return ds.ConnectedTime != nil && ds.ConnectedTime.After(request.CreatedTime)
}

func (ds DataSource) hasStaleData(now time.Time) bool {
	return ds.LatestDataTime != nil && now.Sub(*ds.LatestDataTime) > StaleDuration
}

func (r ConnectionRequest) isExpired(now time.Time) bool {
	return now.After(r.ExpiresAt())
}

func (r ConnectionRequest) isStale(now time.Time) bool {
	return now.Sub(r.CreatedTime) > StaleDuration
}
