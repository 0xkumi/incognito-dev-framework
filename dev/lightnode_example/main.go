package main

import (
	"fmt"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/incognito-chain/blockchain"
)

func main() {
	node := devframework.NewAppNode("fullnode", devframework.MainNetParam, true, false)
	var OnNewShardBlock = func(blk interface{}) {
		blkShard := blk.(*blockchain.ShardBlock)
		fmt.Println("Shard", blkShard.GetShardID(), blkShard.GetHeight())
	}
	node.OnInserted(devframework.BLK_SHARD, OnNewShardBlock)
	select {}
}
