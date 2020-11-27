package testsuite

import (
	"encoding/json"
	"fmt"
	"testing"

	F "github.com/0xkumi/incognito-dev-framework"
	"github.com/0xkumi/incognito-dev-framework/account"
	"github.com/0xkumi/incognito-dev-framework/rpcclient"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/transaction"
)

func Test_SendTX(t *testing.T) {
	sim := F.NewStandaloneSimulation("newsim", F.Config{
		ChainParam: F.NewChainParam(F.ID_TESTNET2).SetActiveShardNumber(2),
		DisableLog: true,
	})
	sim.GenerateBlock().NextRound()
	acc1 := sim.NewAccountFromShard(1)
	acc2 := sim.NewAccountFromShard(0)

	_, err := sim.RPC.API_SendTxPRV(sim.GenesisAccount.PrivateKey, map[string]uint64{
		acc1.PaymentAddress: 1000,
		acc2.PaymentAddress: 3000,
	}, -1, true)
	if err != nil {
		panic(err)
	}

	sim.GenerateBlock(F.Hook{
		Validation: func(chainID int, block common.BlockInterface, doValidation func(common.BlockInterface) error) {
			fmt.Println("PreValidation block", 0)
			if chainID == 0 && block.GetHeight() == 3 {
				newShardBlock := block.(*blockchain.ShardBlock)
				newShardBlock.Body.Transactions = append(newShardBlock.Body.Transactions, newShardBlock.Body.Transactions[0])
				totalTxsFee := make(map[common.Hash]uint64)
				for _, tx := range newShardBlock.Body.Transactions {
					totalTxsFee[*tx.GetTokenID()] += tx.GetTxFee()
					txType := tx.GetType()
					if txType == common.TxCustomTokenPrivacyType {
						txCustomPrivacy := tx.(*transaction.TxCustomTokenPrivacy)
						totalTxsFee[*txCustomPrivacy.GetTokenID()] = txCustomPrivacy.GetTxFeeToken()
					}
				}
				newShardBlock.Header.TotalTxsFee = totalTxsFee
				merkleRoots := blockchain.Merkle{}.BuildMerkleTreeStore(newShardBlock.Body.Transactions)
				merkleRoot := &common.Hash{}
				if len(merkleRoots) > 0 {
					merkleRoot = merkleRoots[len(merkleRoots)-1]
				}
				crossTransactionRoot, err := blockchain.CreateMerkleCrossTransaction(newShardBlock.Body.CrossTransactions)
				if err != nil {
					fmt.Println(err)
				}
				_, shardTxMerkleData := blockchain.CreateShardTxRoot(newShardBlock.Body.Transactions)
				newShardBlock.Header.TxRoot = *merkleRoot
				newShardBlock.Header.ShardTxRoot = shardTxMerkleData[len(shardTxMerkleData)-1]
				newShardBlock.Header.CrossTransactionRoot = *crossTransactionRoot
			}

			err = doValidation(block)
			fmt.Println("PostValidation block", 0, err)
		},
		Insert: func(chainID int, block common.BlockInterface, doInsert func(common.BlockInterface) error) {
			doInsert(block)
			if chainID == 0 {
				bl1, _ := sim.RPC.API_GetBalance(sim.GenesisAccount)
				fmt.Println(bl1)
				bl2, _ := sim.RPC.API_GetBalance(acc1)
				fmt.Println(bl2)
				bl3, _ := sim.RPC.API_GetBalance(acc2)
				fmt.Println(bl3)
				fmt.Printf("%+v", block.(*blockchain.ShardBlock).Body)

			}

		},
	})
	sim.NextRound()

	for i := 0; i < 10; i++ {
		sim.GenerateBlock(F.Hook{
			Insert: func(chainID int, block common.BlockInterface, doInsert func(common.BlockInterface) error) {
				if chainID == -1 {
					//fmt.Printf("%+v %+v", block.(*blockchain.BeaconBlock).GetHeight(), block.(*blockchain.BeaconBlock).Body.ShardState)
					doInsert(block)
				} else {
					doInsert(block)
				}

			},
		}).NextRound()
	}

	balance, _ := sim.RPC.API_GetBalance(acc1)
	fmt.Printf("%+v", balance)

}

func Test_CrossShard(t *testing.T) {
	node := F.NewStandaloneSimulation("newsim", F.Config{
		ChainParam: F.NewChainParam(F.ID_TESTNET2).SetActiveShardNumber(2),
		DisableLog: true,
	})

	node.GenerateBlock().NextRound()

	acc1 := node.NewAccountFromShard(1)
	node.ShowBalance(acc1)

	node.SendPRV(node.GenesisAccount, acc1, 1000)
	for i:=0; i < 10; i++ {
		node.GenerateBlock().NextRound()
		node.SendPRV(node.GenesisAccount, acc1, 1000)
		node.ApplyChain(0,1).GenerateBlock().NextRound()
	}

	node.ShowBalance(acc1)
	fmt.Println(node.GetBlockchain().BeaconChain.GetAllView())
	node.Pause()

	node.GenerateBlock().NextRound()
	node.GenerateBlock().NextRound()
	node.GenerateBlock().NextRound()
	node.GenerateBlock().NextRound()
	node.GenerateBlock().NextRound()
	node.GenerateBlock().NextRound()
	node.ShowBalance(acc1)
}
func Test_StakeFlow1(t *testing.T) {
	sim := F.NewStandaloneSimulation("newsim", F.Config{
		ChainParam: F.NewChainParam(F.ID_TESTNET2).SetActiveShardNumber(2).SetMaxShardCommitteeSize(5),
		DisableLog: true,
	})
	sim.GenerateBlock().NextRound()
	miner1 := sim.NewAccountFromShard(0)

	sim.RPC.API_SendTxPRV(sim.GenesisAccount.PrivateKey, map[string]uint64{miner1.PaymentAddress: 1000000}, -1, true)
	for i := 0; i < 4; i++ {
		sim.GenerateBlock().NextRound()
	}

	sim.GenerateBlock().NextRound()
	minerCm1, _ := account.BuildCommitteePubkey(miner1.PrivateKey, sim.GenesisAccount.PaymentAddress)
	stake1 := rpcclient.StakingTxParam{
		Name:         "miner1",
		CommitteeKey: minerCm1,
		BurnAddr:     sim.GetBlockchain().GetBurningAddress(sim.GetBlockchain().BeaconChain.GetFinalViewHeight()),
		StakerPrk:    sim.GenesisAccount.PrivateKey,
		MinerPrk:     miner1.PrivateKey,
		RewardAddr:   miner1.PaymentAddress,
		StakeShard:   true,
		AutoRestake:  true,
	}

	stakeList := []rpcclient.StakingTxParam{stake1}
	_, err := sim.RPC.API_SendTxStaking(stake1)
	if err != nil {
		panic(err)
	}
	monitorPool := func(oldLen1 int, oldLen2 int, oldLen3 int) (bool, int, int, int) {
		len1 := len(sim.GetBlockchain().BeaconChain.GetShardsWaitingList())
		len2 := 0
		len3 := 0
		for _, sCommittee := range sim.GetBlockchain().BeaconChain.GetShardsPendingList()["bls"] {
			len2 += len(sCommittee)
		}
		for _, sCommittee := range sim.GetBlockchain().BeaconChain.GetAllCommittees()[sim.GetBlockchain().BeaconChain.GetConsensusType()] {
			len3 += len(sCommittee)
		}
		if (oldLen1 != len1) || (oldLen2 != len2) || (oldLen3 != len3) {
			return true, len1, len2, len3
		}
		return false, len1, len2, len3
	}

	viewPool := func() {
		waitingPool := []string{}
		pendingPool := []string{}
		committeePool := []string{}
		for _, stake := range stakeList {
			role, _ := sim.RPC.API_GetPublicKeyRole(stake.CommitteeKey.GetMiningKeyBase58("bls"))
			switch role {
			case "waiting":
				waitingPool = append(waitingPool, stake.Name)
			case "pending":
				pendingPool = append(pendingPool, stake.Name)
			case "committee":
				committeePool = append(committeePool, stake.Name)
			}
		}

		fmt.Println("Waiting Pool:", waitingPool)
		fmt.Println("Pending Pool:", pendingPool)
		fmt.Println("Committee Pool:", committeePool)
	}

	_, l1, l2, l3 := monitorPool(0, 0, 0)
	isChange := false
	for i := 0; i < 100; i++ {
		sim.GenerateBlock().NextRound()
		isChange, l1, l2, l3 = monitorPool(l1, l2, l3)
		if isChange {
			fmt.Println("\n----------------------------------")
			fmt.Println("Beacon Epoch", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetCurrentEpoch())
			fmt.Println("Beacon Height", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetHeight())
			viewPool()
			fmt.Println("----------------------------------")
			isChange = false
		}
	}
	// create miner1 unstake information
	unstake1 := rpcclient.StopStakingParam{
		BurnAddr:  sim.GetBlockchain().GetBurningAddress(sim.GetBlockchain().BeaconChain.GetFinalViewHeight()),
		StakerPrk: sim.GenesisAccount.PrivateKey,
		MinerPrk:  miner1.PrivateKey,
	}

	// send stop staking tx
	if _, err := sim.RPC.API_SendTxStopAutoStake(unstake1); err != nil {
		panic(err)
	}
	fmt.Println("Stopstake staker1 at epoch", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetCurrentEpoch())
	// generate block for tx to add to chain
	sim.GenerateBlock().NextRound()

	// create miner2 stake information
	miner2 := sim.NewAccountFromShard(0)
	minerCm2, _ := account.BuildCommitteePubkey(miner2.PrivateKey, sim.GenesisAccount.PaymentAddress)
	stake2 := rpcclient.StakingTxParam{
		Name:         "miner2",
		CommitteeKey: minerCm2,
		BurnAddr:     sim.GetBlockchain().GetBurningAddress(sim.GetBlockchain().BeaconChain.GetFinalViewHeight()),
		StakerPrk:    sim.GenesisAccount.PrivateKey,
		MinerPrk:     miner2.PrivateKey,
		RewardAddr:   miner2.PaymentAddress,
		StakeShard:   true,
		AutoRestake:  true,
	}
	stakeList = append(stakeList, stake2)

	// send miner2 staking information
	_, err = sim.RPC.API_SendTxStaking(stake2)
	if err != nil {
		panic(err)
	}
	fmt.Println("Stake staker2 at epoch", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetCurrentEpoch())

	for i := 0; i < 100; i++ {
		sim.GenerateBlock().NextRound()
		isChange, l1, l2, l3 = monitorPool(l1, l2, l3)
		if isChange {
			fmt.Println("\n----------------------------------")
			fmt.Println("Beacon Epoch", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetCurrentEpoch())
			fmt.Println("Beacon Height", sim.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetHeight())
			viewPool()
			fmt.Println("----------------------------------")
			isChange = false
		}
	}

	_, err = sim.RPC.API_SendTxWithdrawReward(miner1.PrivateKey, miner1.PaymentAddress)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 5; i++ {
		sim.GenerateBlock().NextRound()
	}

	bl1, _ := sim.RPC.API_GetBalance(miner1)
	fmt.Println(bl1)
}

func Test_PDEFlow(t *testing.T) {
	// F.DisableLog(true)
	sim := F.NewStandaloneSimulation("newsim", F.Config{
		ChainParam: F.NewChainParam(F.ID_TESTNET2).SetActiveShardNumber(2),
		DisableLog: true,
	})
	acc1 := sim.NewAccountFromShard(0)
	_, err := sim.RPC.API_SendTxPRV(sim.GenesisAccount.PrivateKey, map[string]uint64{
		acc1.PaymentAddress: 100000000,
	}, -1, false)
	if err != nil {
		panic(err)
	}
	sim.GenerateBlock().NextRound()
	sim.GenerateBlock().NextRound()

	//Create custom token
	result1, err := sim.RPC.API_SendTxCreateCustomToken(sim.GenesisAccount.PrivateKey, sim.GenesisAccount.PaymentAddress, true, "pTest", "TES", 30000000000)
	if err != nil {
		panic(err)
	}
	fmt.Println(result1.TokenName, result1.TokenID)
	for i := 0; i < 50; i++ {
		sim.GenerateBlock().NextRound()
	}

	bl0, _ := sim.RPC.API_GetBalance(sim.GenesisAccount)
	fmt.Println(bl0)

	// burnAddr := sim.GetBlockchain().GetBurningAddress(sim.GetBlockchain().BeaconChain.GetFinalViewHeight())
	// fmt.Println(burnAddr)
	result2, err := sim.RPC.API_SendTxWithPTokenContributionV2(sim.GenesisAccount, result1.TokenID, 300000000, "testPAIR")
	if err != nil {
		panic(err)
	}

	r2Bytes, _ := json.Marshal(result2)
	fmt.Println(string(r2Bytes))

	for i := 0; i < 10; i++ {
		sim.GenerateBlock().NextRound()
	}

	_, err = sim.RPC.API_SendTxWithPRVContributionV2(sim.GenesisAccount, 100000000000, "testPAIR")
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		sim.GenerateBlock().NextRound()
	}

	r, err := sim.RPC.API_GetPDEState(float64(sim.GetBlockchain().GetBeaconBestState().BeaconHeight))
	if err != nil {
		panic(err)
	}
	rBytes, _ := json.Marshal(r)
	fmt.Println(string(rBytes))
	_, err = sim.RPC.API_SendTxWithPRVCrossPoolTradeReq(acc1, result1.TokenID, 1000000, 1)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		sim.GenerateBlock().NextRound()
	}

	_, err = sim.RPC.API_SendTxWithPTokenCrossPoolTradeReq(sim.GenesisAccount, result1.TokenID, "0000000000000000000000000000000000000000000000000000000000000004", 1000000000, 1)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		sim.GenerateBlock().NextRound()
	}
	fmt.Println("------------------------------------------------------------")
	bl, _ := sim.RPC.API_GetBalance(sim.GenesisAccount)
	fmt.Println("ICO", bl)
	fmt.Println("------------------------------------------------------------")
	bl1, _ := sim.RPC.API_GetBalance(acc1)
	fmt.Println("ACC1", bl1)

	fmt.Println("------------------------------------------------------------")
	r2, err := sim.RPC.API_GetPDEState(float64(sim.GetBlockchain().GetBeaconBestState().BeaconHeight))
	if err != nil {
		panic(err)
	}
	rBytes2, _ := json.Marshal(r2)
	fmt.Println(string(rBytes2))
}
