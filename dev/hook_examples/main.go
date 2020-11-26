package main

import (
	Inc "github.com/0xkumi/incongito-dev-framework"
)

func main() {
	node := Inc.NewStandaloneSimulation("newsim", Inc.Config{
		ShardNumber: 2,
	}, false)

	node.GenerateBlock().NextRound()

	acc1 := node.NewAccountFromShard(0)
	acc2 := node.NewAccountFromShard(0)
	node.ShowBalance(acc1)
	node.ShowBalance(acc2)

	node.SendPRV(node.GenesisAccount, acc1, 1000,acc2, 1000)
	for i := 0; i < 10; i++ {
		node.GenerateBlock().NextRound()
	}

	node.ShowBalance(acc1)
	node.ShowBalance(acc2)


	node.ApplyChain(0).GenerateBlock(Inc.Hook{
		//Create: func(chainID int, doCreate func() (blk common.BlockInterface, err error)) {
		//	doCreate()
		//},
		//Validation: func(chainID int, block common.BlockInterface, doValidation func(blk common.BlockInterface) error) {
		//	doValidation(block)
		//},
		//Insert: func(chainID int, block common.BlockInterface, doInsert func(blk common.BlockInterface) error) {
		//	doInsert(block)
		//},
	})


}
