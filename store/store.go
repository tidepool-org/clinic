package store

import (
	// Built-in Golang packages
	"context" // manage multiple requests
	"go.mongodb.org/mongo-driver/bson/primitive"
	"os" // os.Exit(1) on Error
	"time"
)

var (
	DefaultClinicDatabaseName = "clinic"
	ContextTimeout = time.Duration(20)*time.Second
)

type MongoPagingParams struct {
	Offset int64
	Limit int64
}

type Pagination struct {
	Offset int
	Limit  int
}

func DefaultPagination() Pagination {
	return Pagination{
		Offset: 0,
		Limit: 10,
	}
}

func ObjectIDSFromStringArray(ids []string) []primitive.ObjectID {
	var objectIds []primitive.ObjectID
	for _, id := range ids {
		if objectId, err := primitive.ObjectIDFromHex(id); err == nil {
			objectIds = append(objectIds, objectId)
		}
	}
	return objectIds
}

func NewDbContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), ContextTimeout)
	return ctx
}

// XXX We should use go.common
func GetConnectionString() (string, error) {
	scheme, _ := os.LookupEnv("TIDEPOOL_STORE_SCHEME")
	hosts, _ := os.LookupEnv("TIDEPOOL_STORE_ADDRESSES")
	user, _ := os.LookupEnv("TIDEPOOL_STORE_USERNAME")
	password, _ := os.LookupEnv("TIDEPOOL_STORE_PASSWORD")
	optParams, _ := os.LookupEnv("TIDEPOOL_STORE_OPT_PARAMS")
	ssl, _ := os.LookupEnv("TIDEPOOL_STORE_TLS")


	var cs string
	if scheme != "" {
		cs = scheme + "://"
	} else {
		cs = "mongodb://"
	}

	if user != "" {
		cs += user
		if password != "" {
			cs += ":"
			cs += password
		}
		cs += "@"
	}

	if hosts != "" {
		cs += hosts
		cs += "/"
	} else {
		cs += "localhost/"
	}

	if ssl == "true" {
		cs += "?ssl=true"
	} else {
		cs += "?ssl=false"
	}

	if optParams != "" {
		cs += "&"
		cs += optParams
	}
	return cs, nil
}
