package test

import (
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/sites"
)

func Random() sites.Site {
	return sites.Site{
		Id: primitive.NewObjectID(),
		// This should be random enough to be unique, see the Godoc for the odds.
		Name: uuid.NewString(),
	}
}

func RandomSlice(n int) []sites.Site {
	sites := make([]sites.Site, 0, n)
	for len(sites) < n {
		sites = append(sites, Random())
	}
	return sites
}
