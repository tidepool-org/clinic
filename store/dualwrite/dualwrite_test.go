package dualwrite_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/store/dualwrite"
)

var _ = Describe("Execute", func() {
	var logger *zap.SugaredLogger

	BeforeEach(func() {
		logger = zap.NewNop().Sugar()
	})

	Context("without a queue on the context", func() {
		It("runs the operation immediately", func() {
			ran := false
			dualwrite.Execute(context.Background(), logger, "test", "op", func(ctx context.Context) error {
				ran = true
				return nil
			})
			Expect(ran).To(BeTrue())
		})

		It("swallows errors", func() {
			Expect(func() {
				dualwrite.Execute(context.Background(), logger, "test", "op", func(ctx context.Context) error {
					return fmt.Errorf("mirror failure")
				})
			}).ToNot(Panic())
		})

		It("recovers panics", func() {
			Expect(func() {
				dualwrite.Execute(context.Background(), logger, "test", "op", func(ctx context.Context) error {
					panic("mirror panic")
				})
			}).ToNot(Panic())
		})

		It("runs the operation even when the caller's context is canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			ran := false
			dualwrite.Execute(ctx, logger, "test", "op", func(ctx context.Context) error {
				ran = true
				Expect(ctx.Err()).To(BeNil())
				return nil
			})
			Expect(ran).To(BeTrue())
		})
	})

	Context("with a queue on the context", func() {
		It("defers the operation until the queue is flushed", func() {
			ctx, queue := dualwrite.WithQueue(context.Background())

			ran := 0
			dualwrite.Execute(ctx, logger, "test", "op", func(ctx context.Context) error {
				ran++
				return nil
			})
			Expect(ran).To(Equal(0))

			queue.Flush(context.Background())
			Expect(ran).To(Equal(1))

			// Flushing again must not re-run operations
			queue.Flush(context.Background())
			Expect(ran).To(Equal(1))
		})

		It("drops buffered operations on reset", func() {
			ctx, queue := dualwrite.WithQueue(context.Background())

			ran := false
			dualwrite.Execute(ctx, logger, "test", "op", func(ctx context.Context) error {
				ran = true
				return nil
			})
			queue.Reset()
			queue.Flush(context.Background())
			Expect(ran).To(BeFalse())
		})

		It("preserves the order of operations", func() {
			ctx, queue := dualwrite.WithQueue(context.Background())

			var order []int
			for i := 0; i < 3; i++ {
				i := i
				dualwrite.Execute(ctx, logger, "test", "op", func(ctx context.Context) error {
					order = append(order, i)
					return nil
				})
			}
			queue.Flush(context.Background())
			Expect(order).To(Equal([]int{0, 1, 2}))
		})
	})
})
