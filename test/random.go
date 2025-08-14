package test

import (
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/v2"
	"math/rand"
)

var (
	Faker = faker.NewWithSeed(Source)
	Rand = rand.New(Source)
	Source = rand.NewSource(ginkgo.GinkgoRandomSeed())
)
