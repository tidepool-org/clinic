package test

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/test"
)

func Random() sites.Site {
	id := primitive.NewObjectID()
	name := test.Faker.Lorem().Word()
	return sites.Site{
		Name: name,
		Id:   id,
	}
}

func RandomSlice(n int) []sites.Site {
	uniqNames := make(map[string]struct{})
	sites := make([]sites.Site, 0, n)
	for len(sites) < n {
		s := Random()
		if _, found := uniqNames[s.Name]; !found {
			uniqNames[s.Name] = struct{}{}
			sites = append(sites, s)
		}
	}
	return sites
}
