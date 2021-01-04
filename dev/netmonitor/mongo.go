package main

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	*mongo.Client
}


type BFTMessageCollection struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

func NewBFTMessageCollection(endpoint string, dbName,collectionName string ) (*BFTMessageCollection, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(endpoint))
	if err != nil {
		return nil, err
	}
	err = client.Connect(context.TODO())
	if err != nil {
		return nil, err
	}
	collection := client.Database(dbName).Collection(collectionName)

	//set indexing
	//opts := options.CreateIndexes().SetMaxTime(10 * time.Second)
	//keys := bson.D{{"pri", -1}, {"time_created", 1}}
	//index := mongo.IndexModel{}
	//index.Keys = keys
	//collection.Indexes().CreateOne(context.Background(), index, opts)

	return &BFTMessageCollection{
		client, collection,
		},	nil
}


