package main

import (
	Inc "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	node := Inc.NewStandaloneSimulation("newsim", Inc.Config{
		ChainParam: Inc.NewChainParam(Inc.ID_TESTNET2).SetActiveShardNumber(2),
		DisableLog: false,
	})

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


	node.ApplyChain(0).GenerateBlock().NextRound()


}
