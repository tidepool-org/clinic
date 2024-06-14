package merge

import (
	"context"
	"github.com/dominikbraun/graph"
	"github.com/eapache/queue"
	"github.com/tidepool-org/clinic/patients"
	"slices"
)

const (
	PatientAttributeDOB      = "dob"
	PatientAttributeFullName = "fullName"
	PatientAttributeMRN      = "mrn"
	PatientAttributeUserId   = "userId"

	conflictAttributeKey = "conflict"
)

var (
	attributeGetters = map[string]func(patient patients.Patient) string{
		PatientAttributeDOB:      getDOB,
		PatientAttributeFullName: getFullName,
		PatientAttributeMRN:      getMRN,
		PatientAttributeUserId:   getUserId,
	}
)

type PatientClusters []PatientCluster

func (p PatientClusters) PreventsMerge() bool {
	return false
}

type PatientCluster struct {
	Patients []PatientConflicts
}

type PatientConflicts struct {
	Patient patients.Patient

	// Conflicts is a map from conflict category to user id
	Conflicts map[string][]string
}

type PatientClusterReporter struct {
	graph             graph.Graph[string, patients.Patient]
	patients          []patients.Patient
	targetByAttribute map[string]map[string][]*patients.Patient
}

func NewPatientClusterReporter(pts []patients.Patient) *PatientClusterReporter {
	reporter := &PatientClusterReporter{
		graph:             graph.New(getUserId),
		patients:          pts,
		targetByAttribute: map[string]map[string][]*patients.Patient{},
	}

	for attr, getter := range attributeGetters {
		reporter.targetByAttribute[attr] = make(map[string][]*patients.Patient)
		for _, patient := range pts {
			if value := getter(patient); attr != "" {
				reporter.targetByAttribute[attr][value] = append(reporter.targetByAttribute[attr][value], &patient)
			}
		}
	}

	return reporter
}

func (p *PatientClusterReporter) Plan(ctx context.Context) (PatientClusters, error) {
	return p.GetPatientClusters()
}

func (p *PatientClusterReporter) GetPatientClusters() (PatientClusters, error) {
	for _, patient := range p.patients {
		if err := p.graph.AddVertex(patient); err != nil {
			return nil, err
		}
	}
	for _, patient := range p.patients {
		if err := p.addDuplicateEdges(patient); err != nil {
			return nil, err
		}
	}

	visited := map[string]struct{}{}
	clusters := make([]PatientCluster, 0)
	adjecencyMap, err := p.graph.AdjacencyMap()
	if err != nil {
		return nil, err
	}

	// BFS traversal to get each patient cluster
	for userId := range adjecencyMap {
		cluster := PatientCluster{}
		q := queue.New()
		q.Add(userId)
		for q.Length() != 0 {
			id := q.Remove().(string)
			if _, ok := visited[id]; ok {
				continue
			}

			patient, err := p.graph.Vertex(id)
			if err != nil {
				return nil, err
			}

			conflicts := map[string][]string{}
			for duplicate, edge := range adjecencyMap[id] {
				q.Add(duplicate)
				conflict := edge.Properties.Attributes[conflictAttributeKey]
				conflicts[conflict] = append(conflicts[conflict], duplicate)
			}

			cluster.Patients = append(cluster.Patients, PatientConflicts{
				Patient:   patient,
				Conflicts: conflicts,
			})

			visited[id] = struct{}{}
		}

		if len(cluster.Patients) > 0 {
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (p *PatientClusterReporter) addDuplicateEdges(patient patients.Patient) error {
	userId := getUserId(patient)

	// Duplicate UserId -> Attribute Type
	duplicates := make(map[string][]string)
	for attribute, getter := range attributeGetters {
		value := getter(patient)
		target := p.targetByAttribute[value]
		for _, duplicate := range target[value] {
			if duplicateUserId := getUserId(*duplicate); duplicateUserId != userId {
				duplicates[userId] = append(duplicates[userId], attribute)
			}
		}
	}

	for duplicateUserId, attrs := range duplicates {
		if conflict := getConflict(attrs); conflict != nil {
			if err := p.graph.AddEdge(userId, duplicateUserId, conflict); err != nil {
				return err
			}
		}
	}

	return nil
}

func getConflict(attrs []string) func(*graph.EdgeProperties) {
	var conflictCategory string

	if slices.Contains(attrs, PatientAttributeUserId) {
		conflictCategory = PatientConflictCategoryDuplicateAccounts
	} else if len(attrs) == 1 {
		// We don't need to report DOB only matches
		if attrs[0] == PatientAttributeMRN {
			conflictCategory = PatientConflictCategoryMRNOnlyMatch
		} else if attrs[0] == PatientAttributeFullName {
			conflictCategory = PatientConflictCategoryNameOnlyMatch
		}
	} else if len(attrs) > 1 {
		conflictCategory = PatientConflictCategoryLikelyDuplicateAccounts
	}

	if conflictCategory == "" {
		return nil
	}

	return graph.EdgeAttribute(conflictAttributeKey, conflictCategory)
}

func getDOB(patient patients.Patient) (attr string) {
	if patient.BirthDate != nil {
		attr = *patient.BirthDate
	}
	return
}

func getFullName(patient patients.Patient) (attr string) {
	if patient.FullName != nil {
		attr = *patient.FullName
	}
	return
}

func getMRN(patient patients.Patient) (attr string) {
	if patient.Mrn != nil {
		attr = *patient.Mrn
	}
	return
}

func getUserId(patient patients.Patient) string {
	return *patient.UserId
}
