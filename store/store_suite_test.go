package store_test

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var mongoClient *store.MongoStoreClient

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store Suite")
}

var _ = BeforeSuite(func() {
	store.DatabaseName = "clinic_test"
	mongoClient = store.NewMongoStoreClient(store.MongoHost)

	// Create database and collections
	fmt.Println("Starting suite")
	teardownDatabase(store.DatabaseName)
})

var _ = AfterSuite(func() {
	teardownDatabase(store.DatabaseName)
	fmt.Println("Completed suite")
})

func teardownDatabase(name string) {
	ctx := context.TODO()
	fmt.Println("Dropping old test databases")
	mongoClient.Client.Database(name).Collection(store.ClinicsCollection).Drop(ctx)
	mongoClient.Client.Database(name).Collection(store.ClinicsCliniciansCollection).Drop(ctx)
	mongoClient.Client.Database(name).Collection(store.ClinicsPatientsCollection).Drop(ctx)
	fmt.Println("Finish Database teardown")

}