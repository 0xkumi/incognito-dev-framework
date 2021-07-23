package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
)

func main() {
	chainNetwork := "testnet1"
	chainDataFolder := "chaindata"

	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	os.Setenv("INCOGNITO_CONFIG_MODE_KEY", "file")
	os.Setenv("INCOGNITO_CONFIG_DIR_KEY", pwd+"/config")
	switch chainNetwork {
	case "local":
		os.Setenv("INCOGNITO_NETWORK_KEY", "local")
	case "testnet1", "testnet-1":
		os.Setenv("INCOGNITO_NETWORK_KEY", "testnet")
		os.Setenv("INCOGNITO_NETWORK_VERSION_KEY", "1")
	case "testnet2", "testnet-2":
		os.Setenv("INCOGNITO_NETWORK_KEY", "testnet")
		os.Setenv("INCOGNITO_NETWORK_VERSION_KEY", "2")
	case "mainnet":
		os.Setenv("INCOGNITO_NETWORK_KEY", "mainnet")
	}

	cfg := config.LoadConfig()
	param := config.LoadParam()
	fmt.Println("===========================")
	fmt.Println(cfg.Network())
	fmt.Println(param.ActiveShards)
	fmt.Println("===========================")

	var netw devframework.NetworkParam
	netw.HighwayAddress = cfg.DiscoverPeersAddress
	lightnode := devframework.NewAppNode(chainDataFolder, netw, true, false, false, true)

	time.Sleep(2 * time.Second)
	ShardProcessedState := make(map[byte]uint64)

	for i := 0; i < lightnode.GetBlockchain().GetActiveShardNumber(); i++ {
		statePrefix := fmt.Sprintf("shard-processed-%v", i)
		v, err := lightnode.GetUserDatabase().Get([]byte(statePrefix), nil)
		if err != nil {
			log.Println(err)
			ShardProcessedState[byte(i)] = 1
		} else {
			height, err := strconv.ParseUint(string(v), 0, 64)
			if err != nil {
				panic(err)
			}
			ShardProcessedState[byte(i)] = height
		}
	}

	var OnNewShardBlock = func(bc *blockchain.BlockChain, h common.Hash, height uint64) {
		var blk types.ShardBlock
		blkBytes, err := lightnode.GetUserDatabase().Get(h.Bytes(), nil)
		if err != nil {
			fmt.Println("height", height, h.String())
			panic(err)
		}
		if err := json.Unmarshal(blkBytes, &blk); err != nil {
			panic(err)
		}
		blockHash := blk.Hash().String()
		shardID := blk.GetShardID()
		log.Printf("start processing for block %v height %v shard %v\n", blockHash, blk.GetHeight(), shardID)

		//
		// Block Process go here
		fmt.Println("block txs len", len(blk.Body.Transactions))
		//

		statePrefix := fmt.Sprintf("shard-processed-%v", blk.Header.ShardID)
		err = lightnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
		if err != nil {
			panic(err)
		}
	}

	for i := 0; i < lightnode.GetBlockchain().GetActiveShardNumber(); i++ {
		lightnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
	select {}
}
