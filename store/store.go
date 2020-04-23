package store

import (
	// Built-in Golang packages
	"context" // manage multiple requests
	"fmt" // Println() function
	"os"      // os.Exit(1) on Error
	"reflect" // get an object type
	"time"

	// Official 'mongo-go-driver' packages
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (

	DatabaseName        = ""
	DefaultClinicDatabaseName = "clinic"
	ClinicsCollection   = "clinic"
	ClinicsCliniciansCollection   = "clinicsClinicians"
	ClinicsPatientsCollection   = "clinicsPatients"
	MongoHost           = "mongodb://127.0.0.1:27017"
	DefaultPagingParams = MongoPagingParams{Offset: 0 ,Limit: 10}
)

func init() {
	databaseName, ok := os.LookupEnv("TIDEPOOL_STORE_DATABASE")
	if ok {
		DatabaseName = databaseName
	} else {
		DatabaseName = DefaultClinicDatabaseName
	}
}

//Mongo Storage Client
type MongoStoreClient struct {
	Client *mongo.Client
}

type MongoPagingParams struct {
	Offset int64
	Limit int64
}
func NewMongoStoreClient(mongoHost string) *MongoStoreClient {

	fmt.Println("Creating Mongo Store")
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoHost))
	if err != nil {
		fmt.Println("mongo.NewClient() ERROR:", err)
		os.Exit(1)
	}
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		fmt.Println("mongo.Connect ERROR:", err)
		os.Exit(1)
	}
	fmt.Println("Created Mongo Store Successfully")


	return &MongoStoreClient{
		Client: client,
	}
}

func (d MongoStoreClient) Ping() error {
	ctx := context.TODO()
	return d.Client.Ping(ctx, nil)
}

func (d MongoStoreClient) InsertOne(collection string, document interface{}) error {
	// InsertOne() method Returns mongo.InsertOneResult
	// Access a MongoDB collection through a database
	ctx := context.TODO()
	col := d.Client.Database(DatabaseName).Collection(collection)

	result, insertErr := col.InsertOne(ctx, document)
	if insertErr != nil {
		fmt.Println("InsertOne ERROR:", insertErr)
		os.Exit(1) // safely exit script on error
	} else {
		fmt.Println("InsertOne() result type: ", reflect.TypeOf(result))
		fmt.Println("InsertOne() API result:", result)

		// get the inserted ID string
		newID := result.InsertedID
		fmt.Println("InsertOne() newID:", newID)
		fmt.Println("InsertOne() newID type:", reflect.TypeOf(newID))
	}
	return nil
}

func (d MongoStoreClient) FindOne(collection string, filter interface{}) *mongo.SingleResult {
	ctx := context.TODO()
	fmt.Println("FindOne")

	col := d.Client.Database(DatabaseName).Collection(collection)

	ret := col.FindOne(ctx, filter)
	fmt.Println("Found")
	return ret
}

func (d MongoStoreClient) Find(collection string, filter interface{}, pagingParams *MongoPagingParams) (*mongo.Cursor, error) {
	ctx := context.TODO()
	fmt.Println("FindMany")
	findOptions := options.Find()
	findOptions.SetLimit(pagingParams.Limit)
	findOptions.SetSkip(pagingParams.Offset)

	if pagingParams == nil {
		pagingParams = &DefaultPagingParams
	}
	fmt.Println("print options: ", *findOptions.Limit, *findOptions.Skip)
	fmt.Println("filter: ", filter)


	col := d.Client.Database(DatabaseName).Collection(collection)

	cursor, err := col.Find(ctx, filter, findOptions)
	fmt.Println("FoundMany")
	return cursor, err
}

func (d MongoStoreClient) UpdateOne(collection string, filter interface{}, update interface {}) *mongo.UpdateResult {
	ctx := context.TODO()
	fmt.Println("UpdateOne")

	col := d.Client.Database(DatabaseName).Collection(collection)

	ret, err := col.UpdateOne(ctx, filter, update)
	if err != nil {
		fmt.Println("error on update", err)
	}
	fmt.Println("Updated")
	return ret
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
