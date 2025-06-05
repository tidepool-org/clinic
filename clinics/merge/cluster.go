package merge

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/dominikbraun/graph"
	"github.com/eapache/queue"

	"github.com/tidepool-org/clinic/patients"
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

func (p PatientClusters) Errors() []ReportError {
	return nil
}

type PatientCluster struct {
	Patients []PatientConflicts
}

type PatientConflicts struct {
	// Source patient
	Patient patients.Patient

	// Conflicts is a map from conflict category to target patient user id
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
		targetByAttribute: buildAttributeMap(pts),
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
	adjacencyMap, err := p.graph.AdjacencyMap()
	if err != nil {
		return nil, err
	}

	// BFS traversal to get each patient cluster
	for userId := range adjacencyMap {
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
			for duplicate, edge := range adjacencyMap[id] {
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

		if len(cluster.Patients) > 1 {
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (p *PatientClusterReporter) addDuplicateEdges(patient patients.Patient) error {
	userId := getUserId(patient)
	duplicates := getDuplicates(patient, p.targetByAttribute)

	for duplicateUserId, conflictCategory := range duplicates {
		edgeAttributes := graph.EdgeAttribute(conflictAttributeKey, conflictCategory)
		if err := p.graph.AddEdge(userId, duplicateUserId, edgeAttributes); err != nil && !errors.Is(err, graph.ErrEdgeAlreadyExists) {
			return err
		}
	}

	return nil
}

func getDuplicates(patient patients.Patient, targetByAttribute attributeMap) map[string]string {
	clinicUserId := getClinicUserId(patient)

	// Single user can have more than one duplicate attribute
	// UserId -> List of duplicate attribute types
	userToDuplicateAttributes := make(map[string][]string)
	for attribute, getter := range attributeGetters {
		value := getter(patient)
		target := targetByAttribute[attribute]
		for _, duplicate := range target[value] {
			// Do not add edge between duplicates if the source and target patient are the same
			if duplicateUserId := getClinicUserId(*duplicate); duplicateUserId != clinicUserId {
				userId := getUserId(*duplicate)
				userToDuplicateAttributes[userId] = append(userToDuplicateAttributes[userId], attribute)
			}
		}
	}

	userToConflictCategory := make(map[string]string)
	for userId, attributes := range userToDuplicateAttributes {
		if c := getConflictCategory(attributes); c != nil {
			userToConflictCategory[userId] = *c
		}
	}

	return userToConflictCategory
}

// getConflictCategory returns the best matching conflict category given a list of matching attributes
func getConflictCategory(attrs []string) *string {
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

	return &conflictCategory
}

// Attribute Type -> Attribute Value -> List of patients sharding the values
type attributeMap map[string]map[string][]*patients.Patient

func (a attributeMap) GetPatientsWithMRN(value string) []*patients.Patient {
	return a[PatientAttributeMRN][value]
}

func buildAttributeMap(pts []patients.Patient) attributeMap {
	a := attributeMap{}
	for attr, getter := range attributeGetters {
		a[attr] = make(map[string][]*patients.Patient)
		for _, patient := range pts {
			if value := getter(patient); value != "" {
				a[attr][value] = append(a[attr][value], &patient)
			}
		}
	}
	return a
}

func getDOB(patient patients.Patient) (attr string) {
	if patient.BirthDate != nil {
		attr = *patient.BirthDate
	}
	return
}

func getFullName(patient patients.Patient) (attr string) {
	if patient.FullName != nil {
		attr = strings.ToLower(strings.TrimSpace(*patient.FullName))
	}
	return
}

func getMRN(patient patients.Patient) (attr string) {
	if patient.Mrn != nil {
		attr = strings.ToLower(strings.TrimSpace(*patient.Mrn))
	}
	return
}

func getUserId(patient patients.Patient) string {
	return *patient.UserId
}

func getClinicUserId(patient patients.Patient) string {
	return patient.ClinicId.Hex() + "_" + *patient.UserId
}
