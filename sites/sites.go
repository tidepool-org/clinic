package sites

// Package sites defines clinical sites, intended as tags on patients denoting their
// physical or logical location to ease the management of patient lists.

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Site of a clinic, to which patients can be associated.
type Site struct {
	Id       primitive.ObjectID `bson:"id" json:"id"`
	Name     string             `bson:"name" json:"name"`
	Patients int                `bson:"patients,omitzero" json:"patients,omitzero"`
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

// MaybeRenameSite by adding numbered suffixes, if a duplicate site exists in targetSites.
//
// A site is considered a duplicate when it's not [sites.Site.Equals] to any element of
// targetSites, but has the same name. Returns site.Name when no duplicate is found.
func MaybeRenameSite(site Site, targetSites []Site) (string, error) {
	proposedName := site.Name
	if slices.ContainsFunc(targetSites, site.Equals) {
		return site.Name, nil
	}
	for SiteExistsWithName(targetSites, proposedName) {
		incremented, err := incNumericSuffix(proposedName)
		if err != nil {
			return "", err
		}
		proposedName = incremented
	}
	return proposedName, nil
}

var siteNameSuffix = regexp.MustCompile(` \((\d+)\)$`)

func incNumericSuffix(name string) (string, error) {
	matches := siteNameSuffix.FindStringSubmatch(name)
	if len(matches) != 2 {
		// It has no numeric suffix, so add " (2)".
		return name + " (2)", nil
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		// This can only happen if siteNameSuffix, the regular expression itself, is faulty.
		return "", fmt.Errorf("unable to parse site name suffix: %s", name)
	}
	base := name[:len(name)-len(matches[0])]
	return fmt.Sprintf("%s (%d)", base, n+1), nil
}
