package test

import (
	"math/rand"

	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/v2"
)

var (
	Faker  = faker.NewWithSeed(Source)
	Rand   = rand.New(Source)
	Source = rand.NewSource(ginkgo.GinkgoRandomSeed())
)
