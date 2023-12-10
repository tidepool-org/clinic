package manager_test

import (
	. "github.com/onsi/ginkgo/v2"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"testing"
)

func TestSuite(t *testing.T) {
	test.Test(t)
}

var _ = BeforeSuite(dbTest.SetupDatabase)
var _ = AfterSuite(dbTest.TeardownDatabase)
