package service_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"

	storeTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
)

func TestSuite(t *testing.T) {
	test.Test(t)
}

var _ = BeforeSuite(storeTest.SetupDatabase)
var _ = AfterSuite(storeTest.TeardownDatabase)
