package redox_test

import (
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

func TestSuite(t *testing.T) {
	test.Test(t)
}

var _ = BeforeSuite(dbTest.SetupDatabase)
var _ = AfterSuite(dbTest.TeardownDatabase)
