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
	"os"

	"time"
)

const (
	mongoTimeout = time.Second * 15
)

var (
	Faker    = faker.NewWithSeed(rand.NewSource(ginkgo.GinkgoRandomSeed()))
	database *mongo.Database
)

func SetupDatabase() {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()

	databaseName := fmt.Sprintf("clinic_test_%s_%d", Faker.Letter(), ginkgo.GinkgoParallelProcess())
	Expect(os.Setenv("TIDEPOOL_CLINIC_DATABASE_NAME", databaseName)).To(Succeed())

	config, err := store.NewConfig()
	Expect(err).ToNot(HaveOccurred())

	client, err := store.NewClient(config)
	Expect(err).ToNot(HaveOccurred())

	err = client.Ping(ctx, nil)
	Expect(err).ToNot(HaveOccurred())

	database = client.Database(config.DatabaseName)
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
