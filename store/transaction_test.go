package store_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/store/dualwrite"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

// Pins the post-commit flush semantics of WithTransaction: mirror operations
// executed inside the transaction run only after a successful commit and are
// dropped when the transaction fails.
var _ = Describe("WithTransaction", func() {
	var client *mongo.Client
	logger := zap.NewNop().Sugar()

	BeforeEach(func() {
		client = dbTest.GetTestDatabase().Client()
	})

	It("flushes mirror operations after the transaction commits", func() {
		ran := false
		result, err := store.WithTransaction(context.Background(), client, func(sessCtx mongo.SessionContext) (interface{}, error) {
			dualwrite.Execute(sessCtx, logger, "test", "op", func(ctx context.Context) error {
				ran = true
				return nil
			})
			// Mirror operations must not run before the transaction commits
			Expect(ran).To(BeFalse())
			return "result", nil
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("result"))
		Expect(ran).To(BeTrue())
	})

	It("drops mirror operations when the transaction fails", func() {
		ran := false
		_, err := store.WithTransaction(context.Background(), client, func(sessCtx mongo.SessionContext) (interface{}, error) {
			dualwrite.Execute(sessCtx, logger, "test", "op", func(ctx context.Context) error {
				ran = true
				return nil
			})
			return nil, fmt.Errorf("transaction failure")
		})
		Expect(err).To(HaveOccurred())
		Expect(ran).To(BeFalse())
	})
})
