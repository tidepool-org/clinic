package store

import (
	// Built-in Golang packages
	"context" // manage multiple requests
	"errors"
	"fmt" // Println() function
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	ContextTimeout = time.Duration(20)*time.Second
)

func init() {
	databaseName, ok := os.LookupEnv("TIDEPOOL_STORE_DATABASE")
	if ok {
		DatabaseName = databaseName
	} else {
		DatabaseName = DefaultClinicDatabaseName
	}
}

// Overall storage interface
type StorageInterface interface {
	InsertOne(collection string, document interface{}) (*string, error)
	FindOne(collection string, filter interface{}, data interface{}) error
	Find(collection string, filter interface{}, pagingParams *MongoPagingParams, data interface{}) error
	UpdateOne(collection string, filter interface{}, update interface {}) error
	Update(collection string, filter interface{}, update interface {}) error
	Aggregate(collection string, pipeline []bson.D, data interface {}) error
}

//Mongo Storage Client
type MongoStoreClient struct {
	Client *mongo.Client
}

type MongoPagingParams struct {
	Offset int64
	Limit int64
}

func NewDbContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), ContextTimeout)
	return ctx
}

func NewMongoStoreClient(mongoHost string) *MongoStoreClient {

	fmt.Println("Creating Mongo Store")
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoHost))
	if err != nil {
		fmt.Println("mongo.NewClient() ERROR:", err)
		os.Exit(1)
	}
	//ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	ctx := NewDbContext()
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
	ctx := NewDbContext()
	return d.Client.Ping(ctx, nil)
}

func (d MongoStoreClient) InsertOne(collection string, document interface{}) (*string, error) {
	// InsertOne() method Returns mongo.InsertOneResult
	// Access a MongoDB collection through a database
	col := d.Client.Database(DatabaseName).Collection(collection)

	ctx := NewDbContext()
	result, insertErr := col.InsertOne(ctx, document)
	if insertErr != nil {
		fmt.Println("InsertOne ERROR:", insertErr)
		return nil, insertErr
	}

	fmt.Println("InsertOne() result type: ", reflect.TypeOf(result))
	fmt.Println("InsertOne() API result:", result)

	// get the inserted ID string
	newID := result.InsertedID
	fmt.Println("InsertOne() newID:", newID)
	fmt.Println("InsertOne() newID type:", reflect.TypeOf(newID))

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		newID := oid.Hex()
		return &newID, nil
	} else {
		return nil, errors.New("can not decode database ID")

	}
}

func (d MongoStoreClient) FindOne(collection string, filter interface{}, data interface{}) error {
	fmt.Println("FindOne")

	col := d.Client.Database(DatabaseName).Collection(collection)

	ctx := NewDbContext()
	if err := col.FindOne(ctx, filter).Decode(data); err != nil {
		fmt.Println("Find One error ", err)
		return err
	}
	fmt.Printf("Found: %v\n", data)
	return nil
}

func (d MongoStoreClient) Find(collection string, filter interface{}, pagingParams *MongoPagingParams, data interface{})  error {
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

	ctx := NewDbContext()
	cursor, err := col.Find(ctx, filter, findOptions)
	if err != nil {
		return err
	}
	fmt.Println("FoundMany")
	if err = cursor.All(ctx, data); err != nil {
		return err
	}

	return nil
}

func (d MongoStoreClient) Update(collection string, filter interface{}, update interface {}) error {
	fmt.Println("UpdateOne")

	col := d.Client.Database(DatabaseName).Collection(collection)

	ctx := NewDbContext()
	_, err := col.UpdateMany(ctx, filter, update)
	if err != nil {
		fmt.Println("error on update many", err)
		return err
	}
	fmt.Println("Updated")
	return nil
}

func (d MongoStoreClient) UpdateOne(collection string, filter interface{}, update interface {}) error {
	fmt.Println("UpdateOne")

	col := d.Client.Database(DatabaseName).Collection(collection)

	ctx := NewDbContext()
	_, err := col.UpdateOne(ctx, filter, update)
	if err != nil {
		fmt.Println("error on update one", err)
		return err
	}
	fmt.Println("Updated")
	return nil
}

func (d MongoStoreClient) Aggregate(collection string, pipeline []bson.D , data interface {}) error {
	col := d.Client.Database(DatabaseName).Collection(collection)

	ctx := NewDbContext()
	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	if err = cursor.All(ctx, data); err != nil {
		return err
	}
	fmt.Println("Aggregate:", data)
	return nil
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
