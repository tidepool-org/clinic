package api_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/api"
)

var _ = Describe("Mappers", func() {
	Describe("NewTideReportParams", func() {
		It("keeps nil sites nil", func() {
			params := api.NewTideReportParams(api.TideReportParams{})
			Expect(params.Sites).To(BeNil())
		})

		It("drops empty site ids, so an empty ?sites= value means no filter", func() {
			params := api.NewTideReportParams(api.TideReportParams{
				Sites: &[]api.ObjectIdV1{""},
			})
			Expect(params.Sites).To(BeEmpty())
		})

		It("keeps non-empty site ids while dropping empty ones", func() {
			siteId := "68a1b2c3d4e5f6a7b8c9d0e1"
			params := api.NewTideReportParams(api.TideReportParams{
				Sites: &[]api.ObjectIdV1{"", siteId, ""},
			})
			Expect(params.Sites).To(Equal([]string{siteId}))
		})

		It("maps missing tags to an empty, non-nil slice", func() {
			params := api.NewTideReportParams(api.TideReportParams{})
			Expect(params.Tags).ToNot(BeNil())
			Expect(params.Tags).To(BeEmpty())
		})

		It("drops empty tag ids, so an empty ?tags= value means no filter", func() {
			params := api.NewTideReportParams(api.TideReportParams{
				Tags: []api.ObjectIdV1{""},
			})
			Expect(params.Tags).To(BeEmpty())
		})

		It("keeps non-empty tag ids while dropping empty ones", func() {
			tagId := "68a1b2c3d4e5f6a7b8c9d0e2"
			params := api.NewTideReportParams(api.TideReportParams{
				Tags: []api.ObjectIdV1{"", tagId, ""},
			})
			Expect(params.Tags).To(Equal([]string{tagId}))
		})
	})
})
