package sites

// Package sites defines clinical sites, intended as tags on patients denoting their
// physical or logical location to ease the management of patient lists.

import (
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Site of a clinic, to which patients can be associated.
type Site struct {
	Id   primitive.ObjectID `bson:"id,omitempty" json:"id"`
	Name string             `bson:"name,omitempty" json:"name"`
}

func SiteExistsWithName(sites []Site, name string) bool {
	return slices.ContainsFunc(sites, func(s Site) bool {
		return strings.EqualFold(s.Name, name)
	})
}

func New(name string) *Site {
	return &Site{
		Id:   primitive.NewObjectID(),
		Name: name,
	}
}

// MaxSitesPerClinic limits the sites per clinic, to prevent abuse.
const MaxSitesPerClinic int = 50
