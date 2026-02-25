package outbox_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/outbox"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Outbox Repository", func() {
	var repo outbox.Repository
	var database *mongo.Database
	var collection *mongo.Collection

	BeforeEach(func() {
		database = dbTest.GetTestDatabase()
		collection = database.Collection(outbox.CollectionName)
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		var err error
		repo, err = outbox.NewRepository(database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		_ = collection.Drop(context.Background())
	})

	Describe("Create", func() {
		It("inserts an event and fields persist correctly", func() {
			payload := outbox.SendProviderConnectionEmailPayload{
				ClinicId:     "clinic123",
				ClinicName:   "Test Clinic",
				PatientEmail: "patient@example.com",
				PatientName:  "John Doe",
				ProviderName: "any",
			}

			event, err := outbox.NewEvent(outbox.EventTypeSendProviderConnectionEmail, payload)
			Expect(err).ToNot(HaveOccurred())

			err = repo.Create(context.Background(), event)
			Expect(err).ToNot(HaveOccurred())

			var result outbox.Event
			err = collection.FindOne(context.Background(), bson.M{"eventType": string(outbox.EventTypeSendProviderConnectionEmail)}).Decode(&result)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.Id).ToNot(BeNil())
			Expect(result.EventType).To(Equal(outbox.EventTypeSendProviderConnectionEmail))
			Expect(result.CreatedTime).ToNot(BeZero())
			Expect(result.Payload).ToNot(BeEmpty())

			var decodedPayload outbox.SendProviderConnectionEmailPayload
			Expect(bson.Unmarshal(result.Payload, &decodedPayload)).To(Succeed())
			Expect(decodedPayload.ClinicId).To(Equal("clinic123"))
			Expect(decodedPayload.ClinicName).To(Equal("Test Clinic"))
			Expect(decodedPayload.PatientEmail).To(Equal("patient@example.com"))
			Expect(decodedPayload.PatientName).To(Equal("John Doe"))
			Expect(decodedPayload.ProviderName).To(Equal("any"))
		})
	})
})
