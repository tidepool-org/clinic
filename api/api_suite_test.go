package api_test

import (
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}


// Mock Database
type MockDB struct {
	error string
}

var (
	testId = "testId"
)

var _ = BeforeSuite(func() {
	// Create database and collections
	fmt.Println("Starting suite")
})



func (d MockDB) Ping() error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}
func (d MockDB) InsertOne(collection string, document interface{}) (*string, error) {
	if (d.error != "") {
		return &testId, errors.New(d.error)
	}
	return &testId, nil
}
func (d MockDB) FindOne(collection string, filter interface{}, data interface{}) error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}
func (d MockDB) Find(collection string, filter interface{}, pagingParams *store.MongoPagingParams, data interface{})  error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}
func (d MockDB) UpdateOne(collection string, filter interface{}, update interface {}) error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}
func (d MockDB) Update(collection string, filter interface{}, update interface {}) error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}





func (d MockDB) Aggregate(collection string, pipeline []bson.D, data interface {}) error {
	if (d.error != "") {
		return errors.New(d.error)
	}
	return nil
}
