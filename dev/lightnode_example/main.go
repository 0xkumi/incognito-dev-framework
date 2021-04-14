package main

import (
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain/types"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	node := devframework.NewAppNode("fullnode", devframework.MainNetParam, true, false)
	var OnNewShardBlock = func(blk interface{}) {
		blkShard := blk.(*types.ShardBlock)
		fmt.Println("Shard", blkShard.GetShardID(), blkShard.GetHeight())
	}
	node.OnInserted(devframework.BLK_SHARD, OnNewShardBlock)
	select {}
}
