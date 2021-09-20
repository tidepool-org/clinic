package test

import (
	"context"
	"fmt"
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/config"
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
	Faker    = faker.NewWithSeed(rand.NewSource(config.GinkgoConfig.RandomSeed))
	database *mongo.Database
)

func SetupDatabase() {
	client, err := store.NewClient(mongoTestHost)
	Expect(err).ToNot(HaveOccurred())

	ctx, _ := context.WithTimeout(context.Background(), mongoTimeout)
	err = client.Ping(ctx, nil)
	Expect(err).ToNot(HaveOccurred())

	databaseName := fmt.Sprintf("clinic_test_%s_%d", Faker.Letter(), config.GinkgoConfig.ParallelNode)
	database = client.Database(databaseName)
}

func TeardownDatabase() {
	Expect(database).ToNot(BeNil())
	err := database.Drop(context.Background())
	Expect(err).ToNot(HaveOccurred())

	ctx, _ := context.WithTimeout(context.Background(), mongoTimeout)
	Expect(database.Client().Disconnect(ctx)).ToNot(HaveOccurred())
	database = nil
}

func GetTestDatabase() *mongo.Database {
	Expect(database).ToNot(BeNil())
	return database
}
