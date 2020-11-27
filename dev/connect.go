package main

import (
	"fmt"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func main() {
	node := devframework.NewAppNode("devnode", devframework.MainNetParam, true)
	node.DisableChainLog(true)

	//update blkHeight that want to process from
	//blkHeight = -1, start from current best view height
	//blkHeight > 1 traverse block from particular height
	fromBlkHeight := int64(-1)

	//event processing function
	var OnNewFinalState = func(bc *blockchain.BlockChain, blkHash common.Hash, blkHeight uint64) {
		//Your process here
		fmt.Println("new state here ", blkHash.String(), blkHeight)
		//Note: to build statedb for this block height:  get rootHash from this blkHash -> build statedb

		//example build beacon feature root hash from blkHash
		beaconRootHash, _ := blockchain.GetBeaconRootsHashByBlockHash(bc.GetBeaconChainDatabase(), blkHash)
		stateDB, _ := statedb.NewWithPrefixTrie(beaconRootHash.FeatureStateDBRootHash, statedb.NewDatabaseAccessWarper(bc.GetBeaconChainDatabase()))
		_ = stateDB
		//For consistence, Save blkHeight somewhere, incase we restart this program and dont want to process "already processed" block
	}

	//listen to event
	node.OnNewBlockFromParticularHeight(-1, fromBlkHeight, true, OnNewFinalState)
	select {}
}
