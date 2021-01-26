package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"

	"github.com/incognitochain/incognito-chain/wire"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type MongoClient struct {
	*mongo.Client
}


type bftMessageDB struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}


func NewBFTMessageCollection(endpoint string, dbName,collectionName string ) (*bftMessageDB, error) {
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
	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)
	keys := bson.D{{"pri", -1}, {"time_created", 1}}
	index := mongo.IndexModel{}
	index.Keys = keys
	collection.Indexes().CreateOne(context.Background(), index, opts)

	return &bftMessageDB{
		client, collection,
		},	nil
}



func (s *bftMessageDB) Save(msg *wire.MessageBFT) (err error){
	msgVersion := GetMsgVersion(msg)
	if msgVersion < 1 || msgVersion > 3  {
		fmt.Println("Cannot find message version for", string(msg.Content))
	}

	var doc map[string]interface{}
	if msg.Type == "propose" {
		doc,err = GetProposeDocument(msg, msgVersion)
		if err != nil {
			fmt.Println("propose",err)
			return err
		}
	}

	if msg.Type == "vote" {
		doc,err = GetVoteDocument(msg, msgVersion)
		if err != nil {
			fmt.Println("vote",err)
			return err
		}
	}
	doc["receiveTime"] = int64(time.Now().UnixNano() / int64(time.Millisecond))
	doc["version"] = msgVersion
	_,err = s.Collection.InsertOne(context.TODO(), doc)
	return err
}

type BFTMessageDB interface {
	Save(msg *wire.MessageBFT) error
}

func GetProposeDocument(msg *wire.MessageBFT, version int) (map[string]interface{}, error){
	doc := map[string]interface{}{
		"Type": msg.Type,
		"Chain":msg.ChainKey,
		"Timestamp": msg.Timestamp,
		"Timeslot": msg.TimeSlot,
		"PeerID": msg.PeerID,
	}
	if version == 1 {

	}
	if version == 2 {
		var bftPropose blsbftv2.BFTPropose
		err := json.Unmarshal(msg.Content, &bftPropose)
		if err != nil {
			return nil, err
		}
		if msg.ChainKey == "beacon" {
			var beaconBlk blockchain.BeaconBlock
			err := json.Unmarshal(bftPropose.Block, &beaconBlk)
			if err != nil {
				return nil, err
			}
			doc["BlockData"] = string(bftPropose.Block)
			doc["BlockHash"] = beaconBlk.Hash().String()
			doc["BlockHeight"] = beaconBlk.GetHeight()
			doc["Epoch"] = beaconBlk.GetCurrentEpoch()
			doc["CommitteePubkey"] = beaconBlk.Header.ProducerPubKeyStr
		} else {
			var shardBlk blockchain.ShardBlock
			err := json.Unmarshal(bftPropose.Block, &shardBlk)
			if err != nil {
				return nil, err
			}
			doc["BlockData"] = string(bftPropose.Block)
			doc["BlockHash"] = shardBlk.Hash().String()
			doc["BlockHeight"] = shardBlk.GetHeight()
			doc["Epoch"] = shardBlk.GetCurrentEpoch()
			doc["CommitteePubkey"] = shardBlk.Header.ProducerPubKeyStr
		}
	}
	return doc, nil
}

func GetVoteDocument(msg *wire.MessageBFT, version int) (map[string]interface{}, error){
	doc := map[string]interface{}{
		"Type": msg.Type,
		"Chain":msg.ChainKey,
		"Timestamp": msg.Timestamp,
		"Timeslot": msg.TimeSlot,
		"PeerID": msg.PeerID,
	}
	if version == 1 {

	}
	if version == 2 {
		var bftVote blsbftv2.BFTVote
		err := json.Unmarshal(msg.Content, &bftVote)
		if err != nil {
			return nil, err
		}
		doc["PrevBlockHash"] = bftVote.PrevBlockHash
		doc["BlockHash"] = bftVote.BlockHash
		doc["MiningPubkey"] = bftVote.Validator
		doc["VoteData"] = string(msg.Content)
	}
	return doc, nil
}