package test

import (
	"context"
	"fmt"
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"math/rand"

	"time"
)

const (
	mongoTestHost = "mongodb://127.0.0.1:27017"
	mongoTimeout  = time.Second * 5
)

var (
	Faker    = faker.NewWithSeed(rand.NewSource(ginkgo.GinkgoRandomSeed()))
	database *mongo.Database
)

func SetupDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()

	client, err := store.NewClient(mongoTestHost)
	Expect(err).ToNot(HaveOccurred())

	err = client.Ping(ctx, nil)
	Expect(err).ToNot(HaveOccurred())

	databaseName := fmt.Sprintf("clinic_test_%s_%d", Faker.Letter(), ginkgo.GinkgoParallelProcess())
	database = client.Database(databaseName)
}

func TeardownDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()

	Expect(database).ToNot(BeNil())
	err := database.Drop(ctx)
	Expect(err).ToNot(HaveOccurred())

	Expect(database.Client().Disconnect(ctx)).ToNot(HaveOccurred())
	database = nil
}

func GetTestDatabase() *mongo.Database {
	Expect(database).ToNot(BeNil())
	return database
}
