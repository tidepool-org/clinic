package redox_test

import (
	"context"
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/redox"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"net/http"
)

var _ = Describe("Redox", func() {
	var database *mongo.Database
	var collection *mongo.Collection
	var handler redox.Redox

	BeforeEach(func() {
		database = dbTest.GetTestDatabase()
		collection = database.Collection("redox")
		config := redox.Config{
			VerificationToken: "super-secret-token",
		}
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		var err error
		handler, err = redox.NewHandler(config, database, zap.NewNop().Sugar(), lifecycle)
		Expect(err).ToNot(HaveOccurred())
		Expect(handler).ToNot(BeNil())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		_, err := collection.DeleteMany(nil, bson.M{})
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("VerifyEndpoint", func() {
		It("Returns the challenge when the token is correct", func() {
			challenge := "1234567890"
			result, err := handler.VerifyEndpoint(redox.VerificationRequest{
				VerificationToken: "super-secret-token",
				Challenge:         challenge,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
		})

		It("Returns unauthorized error when the token is incorrect", func() {
			challenge := "1234567890"
			_, err := handler.VerifyEndpoint(redox.VerificationRequest{
				VerificationToken: "incorrect-token",
				Challenge:         challenge,
			})
			Expect(err).To(MatchError(errors.Unauthorized))
		})
	})

	Describe("AuthorizeRequest", func() {
		It("Doesn't return an error when the token is correct", func() {
			req := http.Request{Header: make(http.Header)}
			req.Header.Set("verification-token", "super-secret-token")

			err := handler.AuthorizeRequest(&req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Returns unauthorized error when the token is incorrect", func() {
			req := http.Request{Header: make(http.Header)}
			req.Header.Set("verification-token", "incorrect-token")

			err := handler.AuthorizeRequest(&req)
			Expect(err).To(MatchError(errors.Unauthorized))
		})

		It("Returns unauthorized error when the token is missing", func() {
			req := http.Request{}

			err := handler.AuthorizeRequest(&req)
			Expect(err).To(MatchError(errors.Unauthorized))
		})
	})

	Describe("ProcessEHRMessage", func() {
		It("returns an error when metadata is invalid (missing)", func() {
			ctx := context.Background()
			payload := []byte(`{}`)

			err := handler.ProcessEHRMessage(ctx, payload)
			Expect(err).To(MatchError(errors.BadRequest))

			count, err := collection.CountDocuments(ctx, bson.M{})
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(BeEquivalentTo(0))
		})

		It("inserts the message if the data is valid", func() {
			ctx := context.Background()
			payload, err := test.LoadFixture("test/fixtures/neworder.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(payload).ToNot(HaveLen(0))

			var order redox.NewOrder
			err = json.Unmarshal(payload, &order)
			Expect(err).ToNot(HaveOccurred())

			err = handler.ProcessEHRMessage(ctx, payload)
			Expect(err).ToNot(HaveOccurred())

			env := redox.MessageEnvelope{}
			err = collection.FindOne(ctx, bson.M{
				"meta.Logs.ID": "d9f5d293-7110-461e-a875-3beb089e79f3",
			}).Decode(&env)

			Expect(err).ToNot(HaveOccurred())
			Expect(env.Meta).To(BeEquivalentTo(order.Meta))
		})
	})

})
