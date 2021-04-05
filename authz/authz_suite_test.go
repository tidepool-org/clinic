package authz_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuthz(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authz Suite")
}
