package postgres_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"

	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
)

var _ = BeforeSuite(func() {
	dbTest.SetupDatabase()
	dbTest.SetupPostgres()
})

var _ = AfterSuite(func() {
	dbTest.TeardownDatabase()
	dbTest.TeardownPostgres()
})

func TestSuite(t *testing.T) {
	test.Test(t)
}
