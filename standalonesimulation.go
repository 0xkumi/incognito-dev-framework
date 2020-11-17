package devframework

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkumi/incongito-dev-framework/account"
	"github.com/0xkumi/incongito-dev-framework/mock"
	"github.com/0xkumi/incongito-dev-framework/rpcclient"

	"github.com/incognitochain/incognito-chain/pubsub"

	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/syncker"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incdb"
	_ "github.com/incognitochain/incognito-chain/incdb/lvdb"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	bnbrelaying "github.com/incognitochain/incognito-chain/relaying/bnb"
	btcrelaying "github.com/incognitochain/incognito-chain/relaying/btc"
	"github.com/incognitochain/incognito-chain/rpcserver"
	"github.com/incognitochain/incognito-chain/transaction"
)

type Config struct {
	ShardNumber   int
	RoundInterval int
}

type HookCreate func(chain interface{}, doCreate func(time time.Time) (blk common.BlockInterface, err error))

type Hook struct {
	Create     func(chainID int, doCreate func(time time.Time) (blk common.BlockInterface, err error))
	Validation func(chainID int, block common.BlockInterface, doValidation func(blk common.BlockInterface) error)
	Insert     func(chainID int, block common.BlockInterface, doInsert func(blk common.BlockInterface) error)
}
type SimulationEngine struct {
	config  Config
	simName string
	timer   *TimeEngine

	//for account manager
	accountSeed       string
	accountGenHistory map[int]int
	committeeAccount  map[int][]account.Account
	GenesisAccount    account.Account

	//blockchain dependency object
	Network     *HighwayConnection
	param       *blockchain.Params
	bc          *blockchain.BlockChain
	ps          *pubsub.PubSubManager
	consensus   mock.ConsensusInterface
	txpool      *mempool.TxPool
	temppool    *mempool.TxPool
	btcrd       *mock.BTCRandom
	syncker     *syncker.SynckerManager
	server      *mock.Server
	cPendingTxs chan metadata.Transaction
	cRemovedTxs chan metadata.Transaction
	rpcServer   *rpcserver.RpcServer
	cQuit       chan struct{}

	RPC               *rpcclient.RPCClient
	listennerRegister map[int][]func(msg interface{})
}

func (sim *SimulationEngine) NewAccountFromShard(sid int) account.Account {
	lastID := sim.accountGenHistory[sid]
	lastID++
	sim.accountGenHistory[sid] = lastID
	acc, _ := account.GenerateAccountByShard(sid, lastID, sim.accountSeed)
	return *acc
}

func (sim *SimulationEngine) NewAccount() account.Account {
	lastID := sim.accountGenHistory[0]
	lastID++
	sim.accountGenHistory[0] = lastID
	acc, _ := account.GenerateAccountByShard(0, lastID, sim.accountSeed)
	return *acc
}

func (sim *SimulationEngine) init() {
	simName := sim.simName
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	initLogRotator(filepath.Join(path, simName+".log"))
	dbLogger.SetLevel(common.LevelTrace)
	blockchainLogger.SetLevel(common.LevelTrace)
	bridgeLogger.SetLevel(common.LevelTrace)
	rpcLogger.SetLevel(common.LevelTrace)
	rpcServiceLogger.SetLevel(common.LevelTrace)
	rpcServiceBridgeLogger.SetLevel(common.LevelTrace)
	transactionLogger.SetLevel(common.LevelTrace)
	privacyLogger.SetLevel(common.LevelTrace)
	mempoolLogger.SetLevel(common.LevelTrace)

	//setup param
	blockchain.SetupParam()
	activeNetParams := &blockchain.ChainTest2Param
	activeNetParams.ActiveShards = sim.config.ShardNumber
	common.MaxShardNumber = sim.config.ShardNumber
	activeNetParams.MaxShardCommitteeSize = 5
	for i := 0; i < activeNetParams.MinBeaconCommitteeSize; i++ {
		acc := sim.NewAccountFromShard(-1)
		sim.committeeAccount[-1] = append(sim.committeeAccount[-1], acc)
		activeNetParams.GenesisParams.PreSelectBeaconNodeSerializedPubkey = append(activeNetParams.GenesisParams.PreSelectBeaconNodeSerializedPubkey, acc.SelfCommitteePubkey)
		activeNetParams.GenesisParams.PreSelectBeaconNodeSerializedPaymentAddress = append(activeNetParams.GenesisParams.PreSelectBeaconNodeSerializedPaymentAddress, acc.PaymentAddress)
	}
	for i := 0; i < activeNetParams.ActiveShards; i++ {
		for a := 0; a < activeNetParams.MinShardCommitteeSize; a++ {
			acc := sim.NewAccountFromShard(i)
			sim.committeeAccount[i] = append(sim.committeeAccount[i], acc)
			activeNetParams.GenesisParams.PreSelectShardNodeSerializedPubkey = append(activeNetParams.GenesisParams.PreSelectShardNodeSerializedPubkey, acc.SelfCommitteePubkey)
			activeNetParams.GenesisParams.PreSelectShardNodeSerializedPaymentAddress = append(activeNetParams.GenesisParams.PreSelectShardNodeSerializedPaymentAddress, acc.PaymentAddress)
		}
	}
	sim.GenesisAccount = sim.NewAccount()
	initTxs := createGenesisTx([]account.Account{sim.GenesisAccount})
	activeNetParams.GenesisParams.InitialIncognito = initTxs
	activeNetParams.CreateGenesisBlocks()

	//init blockchain
	bc := blockchain.BlockChain{}

	sim.timer.init(activeNetParams.GenesisBeaconBlock.Header.Timestamp + 10)

	cs := mock.Consensus{}
	txpool := mempool.TxPool{}
	temppool := mempool.TxPool{}
	btcrd := mock.BTCRandom{} // use mock for now
	sync := syncker.NewSynckerManager()
	server := mock.Server{
		BlockChain: &bc,
	}
	ps := pubsub.NewPubSubManager()
	fees := make(map[byte]*mempool.FeeEstimator)
	for i := byte(0); i < byte(activeNetParams.ActiveShards); i++ {
		fees[i] = mempool.NewFeeEstimator(
			mempool.DefaultEstimateFeeMaxRollback,
			mempool.DefaultEstimateFeeMinRegisteredBlocks,
			0)
	}
	cPendingTxs := make(chan metadata.Transaction, 500)
	cRemovedTxs := make(chan metadata.Transaction, 500)
	cQuit := make(chan struct{})
	blockgen, err := blockchain.NewBlockGenerator(&txpool, &bc, sync, cPendingTxs, cRemovedTxs)
	if err != nil {
		panic(err)
	}

	db, err := incdb.OpenMultipleDB("leveldb", filepath.Join("./"+simName, "database"))
	// Create db and use it.
	if err != nil {
		panic(err)
	}

	//listenFunc := net.Listen
	//listener, err := listenFunc("tcp", "0.0.0.0:8000")
	//if err != nil {
	//	panic(err)
	//}

	rpcConfig := rpcserver.RpcServerConfig{
		HttpListenters: []net.Listener{nil},
		RPCMaxClients:  1,
		DisableAuth:    true,
		ChainParams:    activeNetParams,
		BlockChain:     &bc,
		Blockgen:       blockgen,
		TxMemPool:      &txpool,
		Server:         &server,
		Database:       db,
	}
	rpcServer := &rpcserver.RpcServer{}
	rpclocal := &LocalRPCClient{rpcServer}

	btcChain, err := getBTCRelayingChain(activeNetParams.BTCRelayingHeaderChainID, "btcchain", simName)
	if err != nil {
		panic(err)
	}
	bnbChainState, err := getBNBRelayingChainState(activeNetParams.BNBRelayingHeaderChainID, simName)
	if err != nil {
		panic(err)
	}

	txpool.Init(&mempool.Config{
		ConsensusEngine: &cs,
		BlockChain:      &bc,
		DataBase:        db,
		ChainParams:     activeNetParams,
		FeeEstimator:    fees,
		TxLifeTime:      100,
		MaxTx:           1000,
		// DataBaseMempool:   dbmp,
		IsLoadFromMempool: false,
		PersistMempool:    false,
		RelayShards:       nil,
		PubSubManager:     ps,
	})
	// serverObj.blockChain.AddTxPool(serverObj.memPool)
	txpool.InitChannelMempool(cPendingTxs, cRemovedTxs)

	temppool.Init(&mempool.Config{
		BlockChain:    &bc,
		DataBase:      db,
		ChainParams:   activeNetParams,
		FeeEstimator:  fees,
		MaxTx:         1000,
		PubSubManager: ps,
	})
	txpool.IsBlockGenStarted = true
	go temppool.Start(cQuit)
	go txpool.Start(cQuit)

	err = bc.Init(&blockchain.Config{
		BTCChain:        btcChain,
		BNBChainState:   bnbChainState,
		ChainParams:     activeNetParams,
		DataBase:        db,
		MemCache:        memcache.New(),
		BlockGen:        blockgen,
		TxPool:          &txpool,
		TempTxPool:      &temppool,
		Server:          &server,
		Syncker:         sync,
		PubSubManager:   ps,
		FeeEstimator:    make(map[byte]blockchain.FeeEstimator),
		RandomClient:    &btcrd,
		ConsensusEngine: &cs,
		GenesisParams:   blockchain.GenesisParam,
	})
	if err != nil {
		panic(err)
	}
	bc.InitChannelBlockchain(cRemovedTxs)

	sim.param = activeNetParams
	sim.bc = &bc
	sim.consensus = &cs
	sim.txpool = &txpool
	sim.temppool = &temppool
	sim.btcrd = &btcrd
	sim.syncker = sync
	sim.server = &server
	sim.cPendingTxs = cPendingTxs
	sim.cRemovedTxs = cRemovedTxs
	sim.rpcServer = rpcServer
	sim.RPC = rpcclient.NewRPCClient(rpclocal)
	sim.cQuit = cQuit
	sim.listennerRegister = make(map[int][]func(msg interface{}))
	sim.ps = ps
	rpcServer.Init(&rpcConfig)
	go func() {
		for {
			select {
			case <-cQuit:
				return
			case <-cRemovedTxs:
			}
		}
	}()
	go blockgen.Start(cQuit)

	sim.startPubSub()

	//init syncker
	sim.syncker.Init(&syncker.SynckerManagerConfig{Blockchain: sim.bc})
}

func (sim *SimulationEngine) startPubSub() {
	go sim.ps.Start()
	go func() {
		_, subChan, err := sim.ps.RegisterNewSubscriber(pubsub.BeaconBeststateTopic)
		if err != nil {
			panic("something wrong with subscriber")
		}
		for {
			event := <-subChan
			for _, f := range sim.listennerRegister[BLK_BEACON] {
				f(event.Value)
			}
		}
	}()

	go func() {
		_, subChan, err := sim.ps.RegisterNewSubscriber(pubsub.ShardBeststateTopic)
		if err != nil {
			panic("something wrong with subscriber")
		}
		for {
			event := <-subChan
			for _, f := range sim.listennerRegister[BLK_SHARD] {
				f(event.Value)
			}
		}
	}()
}

func (sim *SimulationEngine) ConnectNetwork(highwayAddr string, relayShards []byte) {
	config := HighwayConnectionConfig{
		"127.0.0.1",
		19876,
		"2.0.0",
		highwayAddr,
		"",
		sim.consensus,
		sim.syncker,
		relayShards,
	}
	sim.Network = NewHighwayConnection(config)
	sim.Network.Connect()

	sim.syncker.Init(&syncker.SynckerManagerConfig{Network: sim.Network.conn, Blockchain: sim.bc, Consensus: sim.consensus})
	sim.syncker.Start()

}

func getBTCRelayingChain(btcRelayingChainID string, btcDataFolderName string, dataFolder string) (*btcrelaying.BlockChain, error) {
	relayingChainParams := map[string]*chaincfg.Params{
		blockchain.TestnetBTCChainID:  btcrelaying.GetTestNet3Params(),
		blockchain.Testnet2BTCChainID: btcrelaying.GetTestNet3ParamsForInc2(),
		blockchain.MainnetBTCChainID:  btcrelaying.GetMainNetParams(),
	}
	relayingChainGenesisBlkHeight := map[string]int32{
		blockchain.TestnetBTCChainID:  int32(1833130),
		blockchain.Testnet2BTCChainID: int32(1833130),
		blockchain.MainnetBTCChainID:  int32(634140),
	}
	return btcrelaying.GetChainV2(
		filepath.Join("./"+dataFolder, btcDataFolderName),
		relayingChainParams[btcRelayingChainID],
		relayingChainGenesisBlkHeight[btcRelayingChainID],
	)
}

func getBNBRelayingChainState(bnbRelayingChainID string, dataFolder string) (*bnbrelaying.BNBChainState, error) {
	bnbChainState := new(bnbrelaying.BNBChainState)
	err := bnbChainState.LoadBNBChainState(
		filepath.Join("./"+dataFolder, "bnbrelayingv3"),
		bnbRelayingChainID,
	)
	if err != nil {
		log.Printf("Error getBNBRelayingChainState: %v\n", err)
		return nil, err
	}
	return bnbChainState, nil
}

func (sim *SimulationEngine) Pause() {
	fmt.Print("Simulation pause! Press Enter to continue ...")
	var input string
	fmt.Scanln(&input)
	fmt.Print("\n")
}

func (sim *SimulationEngine) PrintBlockChainInfo() {
	fmt.Println("Beacon Chain:")

	fmt.Println("Shard Chain:")
}

//life cycle of a block generation process:
//PreCreate -> PreValidation -> PreInsert ->
func (sim *SimulationEngine) GenerateBlock(args ...interface{}) *SimulationEngine {
	var chainArray = []int{-1}
	for i := 0; i < sim.config.ShardNumber; i++ {
		chainArray = append(chainArray, i)
	}
	var h *Hook

	for _, arg := range args {
		switch arg.(type) {
		case Hook:
			hook := arg.(Hook)
			h = &hook
		case *Execute:
			exec := arg.(*Execute)
			chainArray = exec.appliedChain
		}
	}

	//beacon
	chain := sim.bc
	var block common.BlockInterface = nil
	var err error

	//Create blocks for apply chain
	for _, chainID := range chainArray {
		if h != nil && h.Create != nil {
			h.Create(chainID, func(time time.Time) (blk common.BlockInterface, err error) {
				if chainID == -1 {
					block, err = chain.BeaconChain.CreateNewBlock(2, "", 1, sim.timer.Now())
					if err != nil {
						block = nil
						return nil, err
					}
					block.(mock.BlockValidation).AddValidationField("test")
					return block, nil
				} else {
					block, err = chain.ShardChain[byte(chainID)].CreateNewBlock(2, "", 1, sim.timer.Now())
					if err != nil {
						return nil, err
					}
					block.(mock.BlockValidation).AddValidationField("test")
					return block, nil
				}
			})
		} else {
			if chainID == -1 {
				block, err = chain.BeaconChain.CreateNewBlock(2, "", 1, sim.timer.Now())
				if err != nil {
					block = nil
					fmt.Println("NewBlockError", err)
				}
				block.(mock.BlockValidation).AddValidationField("test")
			} else {
				block, err = chain.ShardChain[byte(chainID)].CreateNewBlock(2, "", 1, sim.timer.Now())
				if err != nil {
					block = nil
					fmt.Println("NewBlockError", err)
				}
				block.(mock.BlockValidation).AddValidationField("test")
			}
		}

		//Validation
		if h != nil && h.Validation != nil {
			h.Validation(chainID, block, func(blk common.BlockInterface) (err error) {
				if blk == nil {
					return errors.New("No block for validation")
				}
				if chainID == -1 {
					err = chain.VerifyPreSignBeaconBlock(blk.(*blockchain.BeaconBlock), true)
					if err != nil {
						return err
					}
					return nil
				} else {
					err = chain.VerifyPreSignShardBlock(block.(*blockchain.ShardBlock), byte(chainID))
					if err != nil {
						return err
					}
					return nil
				}
			})
		} else {
			if block == nil {
				fmt.Println("VerifyBlockErr no block")
			} else {
				if chainID == -1 {
					err = chain.VerifyPreSignBeaconBlock(block.(*blockchain.BeaconBlock), true)
					if err != nil {
						fmt.Println("VerifyBlockErr", err)
					}
				} else {
					err = chain.VerifyPreSignShardBlock(block.(*blockchain.ShardBlock), byte(chainID))
					if err != nil {
						fmt.Println("VerifyBlockErr", err)
					}
				}
			}

		}

		//Insert
		if h != nil && h.Insert != nil {
			h.Insert(chainID, block, func(blk common.BlockInterface) (err error) {
				if blk == nil {
					return errors.New("No block for insert")
				}
				if chainID == -1 {
					err = chain.InsertBeaconBlock(blk.(*blockchain.BeaconBlock), true)
					if err != nil {
						return err
					}
					return
				} else {
					err = chain.InsertShardBlock(blk.(*blockchain.ShardBlock), true)
					if err != nil {
						return err
					} else {
						crossX := block.(*blockchain.ShardBlock).CreateAllCrossShardBlock(sim.config.ShardNumber)
						for _, blk := range crossX {
							fmt.Println("add cross shard block into system")
							sim.Pause()
							sim.syncker.InsertCrossShardBlock(blk)
						}
					}
					return
				}
			})
		} else {
			if block == nil {
				fmt.Println("InsertBlkErr no block")
			} else {
				if chainID == -1 {
					err = chain.InsertBeaconBlock(block.(*blockchain.BeaconBlock), true)
					if err != nil {
						fmt.Println("InsertBlkErr", err)
					}
				} else {
					err = chain.InsertShardBlock(block.(*blockchain.ShardBlock), true)
					if err != nil {
						fmt.Println("InsertBlkErr", err)
					} else {
						crossX := block.(*blockchain.ShardBlock).CreateAllCrossShardBlock(sim.config.ShardNumber)
						for _, blk := range crossX {
							fmt.Println("add cross shard block into system")
							sim.Pause()
							sim.syncker.InsertCrossShardBlock(blk)
						}

					}

				}
			}
		}
	}

	return sim
}

//number of second we want simulation to forward
//default = round interval
func (sim *SimulationEngine) NextRound() {
	sim.timer.Forward(10)
}

func (sim *SimulationEngine) InjectTx(txBase58 string) error {
	rawTxBytes, _, err := base58.Base58Check{}.Decode(txBase58)
	if err != nil {
		return err
	}
	var tx transaction.Tx
	err = json.Unmarshal(rawTxBytes, &tx)
	if err != nil {
		return err
	}
	sim.cPendingTxs <- &tx

	return nil
}

func (sim *SimulationEngine) GetBlockchain() *blockchain.BlockChain {
	return sim.bc
}

func DisableLog(disable bool) {
	disableStdoutLog = disable
}

func (sim *SimulationEngine) GetCommitteePublicKeyState(userPk *incognitokey.CommitteePublicKey) (role string, chainID int) {

	//For Beacon, check in beacon state, if user is in committee
	for _, v := range sim.bc.BeaconChain.GetCommittee() {
		if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) {
			return common.CommitteeRole, -1
		}
	}
	for _, v := range sim.bc.BeaconChain.GetPendingCommittee() {
		if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) {
			return common.PendingRole, -1
		}
	}

	//For Shard
	shardPendingCommiteeFromBeaconView := sim.bc.GetBeaconBestState().GetShardPendingValidator()
	shardCommiteeFromBeaconView := sim.bc.GetBeaconBestState().GetShardCommittee()
	shardCandidateFromBeaconView := sim.bc.GetBeaconBestState().GetShardCandidate()
	//check if in committee of any shard
	for _, chain := range sim.bc.ShardChain {
		for _, v := range chain.GetCommittee() {
			if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) { // in shard commitee in shard state
				return common.CommitteeRole, chain.GetShardID()
			}
		}

		for _, v := range chain.GetPendingCommittee() {
			if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) { // in shard pending ommitee in shard state
				return common.PendingRole, chain.GetShardID()
			}
		}
	}

	//check if in committee or pending committee in beacon
	for _, chain := range sim.bc.ShardChain {
		for _, v := range shardPendingCommiteeFromBeaconView[byte(chain.GetShardID())] { //if in pending commitee in beacon state
			if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) {
				return common.PendingRole, chain.GetShardID()
			}
		}

		for _, v := range shardCommiteeFromBeaconView[byte(chain.GetShardID())] { //if in commitee in beacon state, but not in shard
			if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) {
				return common.SyncingRole, chain.GetShardID()
			}
		}
	}

	//if is waiting for assigning
	for _, v := range shardCandidateFromBeaconView {
		if v.IsEqualMiningPubKey(common.BlsConsensus, userPk) {
			return common.WaitingRole, -2
		}
	}

	return "", -2
}

func (s *SimulationEngine) OnReceive(msgType int, f func(msg interface{})) {
	s.Network.listennerRegister[msgType] = append(s.Network.listennerRegister[msgType], f)
}

//note: this function is async, meaning that f function does not lock insert process
func (s *SimulationEngine) OnInserted(blkType int, f func(msg interface{})) {
	s.listennerRegister[blkType] = append(s.listennerRegister[blkType], f)
}

func (s *SimulationEngine) OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64)) {
	chain := s.bc.GetChain(chainID)
	waitingBlkHeight := uint64(blkHeight)
	if blkHeight == -1 {
		if isFinalized {
			waitingBlkHeight = chain.GetFinalView().GetHeight()
		} else {
			waitingBlkHeight = chain.GetBestView().GetHeight()
		}
	}

	go func() {
		for {
			if (isFinalized && chain.GetFinalView().GetHeight() > waitingBlkHeight) || (!isFinalized && chain.GetBestView().GetHeight() > waitingBlkHeight) {
				if chainID == -1 {
					hash, err := s.bc.GetBeaconBlockHashByHeight(chain.GetFinalView(), chain.GetBestView(), waitingBlkHeight)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight)
					}
				} else {
					hash, err := s.bc.GetShardBlockHashByHeight(chain.GetFinalView(), chain.GetBestView(), waitingBlkHeight)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight)
					}
				}
			} else {
				time.Sleep(500 * time.Millisecond)
			}
			waitingBlkHeight++
		}
	}()
}

func (s *SimulationEngine) DisableChainLog() {
	disableStdoutLog = true
}
