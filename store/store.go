package store

import (
	// Built-in Golang packages
	"context" // manage multiple requests
	"fmt" // Println() function
	"go.mongodb.org/mongo-driver/bson"
	"os"      // os.Exit(1) on Error
	"reflect" // get an object type
	"time"

	// Official 'mongo-go-driver' packages
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	DatabaseName = "user"
	CollectionName = "clinic"
	MongoHost = "mongodb://127.0.0.1:27017"
)

//Mongo Storage Client
type MongoStoreClient struct {
	Client *mongo.Client
	Ctx context.Context
	Col *mongo.Collection
}
func NewMongoStoreClient() *MongoStoreClient {

	client, err := mongo.NewClient(options.Client().ApplyURI(MongoHost))
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

	// Access a MongoDB collection through a database
	col := client.Database(DatabaseName).Collection(CollectionName)
	fmt.Println("Collection type:", reflect.TypeOf(col), "\n")


	return &MongoStoreClient{
		Client: client,
		Ctx: ctx,
		Col: col,
	}
}

func (d MongoStoreClient) Ping() error {
	return d.Client.Ping(d.Ctx, nil)
}

func (d MongoStoreClient) InsertOne(document interface{}) error {
	// InsertOne() method Returns mongo.InsertOneResult
	result, insertErr := d.Col.InsertOne(d.Ctx, document)
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

func (d MongoStoreClient) FindOne(filter interface{}) *mongo.SingleResult {
	return d.Col.FindOne(d.Ctx, bson.D{})
}