package sites

// Package sites defines clinical sites, intended as tags on patients denoting their
// physical or logical location to ease the management of patient lists.

import (
	"fmt"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Site of a clinic, to which patients can be associated.
type Site struct {
	Id       primitive.ObjectID `bson:"id" json:"id"`
	Name     string             `bson:"name" json:"name"`
	Patients int                `bson:"-" json:"patients,omitzero"`
}

// Equals compares Name and Id to determine if two sites are equal.
func (s Site) Equals(other Site) bool {
	return s.Name == other.Name && s.Id.Hex() == other.Id.Hex()
}

func (s Site) String() string {
	return fmt.Sprintf("{Id:%s Name:%s}", s.Id.Hex(), s.Name)
}

// GomegaString cuz gomega is annoying and doesn't fall back to a standard fmt.Stringer.
func (s Site) GomegaString() string {
	return fmt.Sprintf("{Id:%s Name:%s}", s.Id.Hex(), s.Name)
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
