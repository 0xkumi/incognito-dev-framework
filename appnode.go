package devframework

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkumi/incognito-dev-framework/mock"
	"github.com/0xkumi/incognito-dev-framework/rpcclient"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	lvdbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/pubsub"
	"github.com/incognitochain/incognito-chain/rpcserver"
	"github.com/incognitochain/incognito-chain/syncker"
)

type AppNodeInterface interface {
	OnReceive(msgType int, f func(msg interface{}))
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
	DisableChainLog(bool)
	GetBlockchain() *blockchain.BlockChain
	GetRPC() *rpcclient.RPCClient
	GetUserDatabase() *leveldb.DB
	// LightNode() LightNodeInterface
}

type LightNodeInterface interface {
}
type NetworkParam struct {
	ChainParam     *blockchain.Params
	HighwayAddress string
}

var (
	MainNetParam = NetworkParam{
		HighwayAddress: "51.91.72.45:9330",
	}
	TestNetParam = NetworkParam{
		HighwayAddress: "45.56.115.6:9330",
	}
	TestNet2Param = NetworkParam{
		HighwayAddress: "74.207.247.250:9330",
	}
)

func NewAppNode(name string, networkParam NetworkParam, isLightNode bool, enableRPC bool) AppNodeInterface {
	// os.RemoveAll(name)
	nodeMode := "full"
	if isLightNode {
		nodeMode = "light"
	}
	sim := &SimulationEngine{
		simName:     name,
		appNodeMode: nodeMode,
	}
	chainParam := &blockchain.Params{}
	switch networkParam {
	case MainNetParam:
		blockchain.IsTestNet = false
		blockchain.IsTestNet2 = false
		blockchain.ReadKey(MainnetKeylist, Mainnetv2Keylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainMainParam
		break
	case TestNetParam:
		blockchain.IsTestNet = true
		blockchain.IsTestNet2 = false
		blockchain.ReadKey(TestnetKeylist, TestnetKeylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainTestParam
		break
	case TestNet2Param:
		blockchain.IsTestNet = true
		blockchain.IsTestNet2 = true
		blockchain.ReadKey(Testnet2Keylist, Testnet2v2Keylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainTest2Param
		break
	default:
		blockchain.IsTestNet = false
		blockchain.IsTestNet2 = false
		chainParam = networkParam.ChainParam
		break
	}
	sim.initNode(chainParam, enableRPC)
	relayShards := []byte{}
	if isLightNode {
		go sim.startLightSyncProcess()
	} else {
		for index := 0; index < common.MaxShardNumber; index++ {
			relayShards = append(relayShards, byte(index))
		}
	}
	sim.ConnectNetwork(networkParam.HighwayAddress, relayShards)
	sim.DisableChainLog(true)
	return sim
}

func (sim *SimulationEngine) initNode(chainParam *blockchain.Params, enableRPC bool) {
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
	activeNetParams := chainParam
	activeNetParams.CreateGenesisBlocks()

	common.MaxShardNumber = activeNetParams.ActiveShards
	//init blockchain
	bc := blockchain.BlockChain{}

	cs := consensus_v2.NewConsensusEngine()
	txpool := mempool.TxPool{}
	temppool := mempool.TxPool{}
	btcrd := mock.BTCRandom{} // use mock for now
	sync := syncker.NewSynckerManager()
	server := mock.Server{}
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

	// pubkey := cs.GetMiningPublicKeys()
	dbpath := filepath.Join("./"+simName, "database")
	db, err := incdb.OpenMultipleDB("leveldb", dbpath)
	// Create db and use it.
	if err != nil {
		panic(err)
	}
	lsList := []net.Listener{}
	if enableRPC {
		listenFunc := net.Listen
		listener, err := listenFunc("tcp", "0.0.0.0:8000")
		if err != nil {
			panic(err)
		}
		lsList = append(lsList, listener)
	}

	rpcConfig := rpcserver.RpcServerConfig{
		HttpListenters: lsList,
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
		ConsensusEngine: cs,
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

	relayShards := []byte{}
	for index := 0; index < common.MaxShardNumber; index++ {
		relayShards = append(relayShards, byte(index))
	}
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
		ConsensusEngine: cs,
		GenesisParams:   blockchain.GenesisParam,
		RelayShards:     relayShards,
	})
	if err != nil {
		panic(err)
	}
	bc.InitChannelBlockchain(cRemovedTxs)

	sim.param = activeNetParams
	sim.bc = &bc
	sim.consensus = cs
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
	if enableRPC {
		go rpcServer.Start()
	}

	//init syncker
	sim.syncker.Init(&syncker.SynckerManagerConfig{Blockchain: sim.bc})

	//init user database
	handles := 256
	cache := 8
	userDBPath := filepath.Join(dbpath, "userdb")
	lvdb, err := leveldb.OpenFile(userDBPath, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*lvdbErrors.ErrCorrupted); corrupted {
		lvdb, err = leveldb.RecoverFile(userDBPath, nil)
	}
	sim.userDB = lvdb
	if err != nil {
		panic(errors.Wrapf(err, "levelvdb.OpenFile %s", userDBPath))
	}
}

func (sim *SimulationEngine) GetRPC() *rpcclient.RPCClient {
	return sim.RPC
}

func (sim *SimulationEngine) startLightSyncProcess() {
	time.Sleep(10 * time.Second)
	fmt.Println("start light sync process")
	sim.lightNodeData.Shards = make(map[byte]*currentShardState)
	for i := 0; i < sim.bc.GetChainParams().ActiveShards; i++ {
		sim.lightNodeData.Shards[byte(i)] = &currentShardState{}
	}
	sim.loadLightShardsState()
	for i := 0; i < sim.bc.GetChainParams().ActiveShards; i++ {
		go sim.syncShardLight(byte(i), sim.lightNodeData.Shards[byte(i)])
	}
}

func (sim *SimulationEngine) loadLightShardsState() {
	for i := 0; i < sim.bc.GetChainParams().ActiveShards; i++ {
		statePrefix := fmt.Sprintf("state-%v", i)
		v, err := sim.userDB.Get([]byte(statePrefix), nil)
		if err != nil && err != leveldb.ErrNotFound {
			panic(err)
		}
		if err == leveldb.ErrNotFound {
			continue
		}
		shardState := &currentShardState{}
		if err := json.Unmarshal(v, shardState); err != nil {
			panic(err)
		}
		sim.lightNodeData.Shards[byte(i)] = shardState
	}
}

func (sim *SimulationEngine) syncShardLight(shardID byte, state *currentShardState) {
	for {
		bestHeight := sim.bc.BeaconChain.GetShardBestViewHeight()[shardID]
		bestHash := sim.bc.BeaconChain.GetShardBestViewHash()[shardID]
		state.BestHeight = bestHeight
		state.BestHash = &bestHash
		blkCh, err := sim.Network.GetShardBlock(int(shardID), state.LocalHeight, state.BestHeight)
		if err != nil && err.Error() != "requester not ready" {
			panic(err)
		}
		if err != nil && err.Error() == "requester not ready" {
			time.Sleep(5 * time.Second)
			continue
		}
		for blk := range blkCh {
			blkBytes, err := json.Marshal(blk)
			if err != nil {
				panic(err)
			}
			blkHash := blk.(*blockchain.ShardBlock).Hash()
			prefix := fmt.Sprintf("s-%v-%v", shardID, blk.GetHeight())
			if err := sim.userDB.Put([]byte(prefix), blkHash.Bytes(), nil); err != nil {
				panic(err)
			}
			if err := sim.userDB.Put(blkHash.Bytes(), blkBytes, nil); err != nil {
				panic(err)
			}
			state.LocalHeight = blk.GetHeight()
			state.LocalHash = blkHash
			fmt.Println("blk", blk.GetHeight())

			go sim.ps.PublishMessage(pubsub.NewMessage(pubsub.NewShardblockTopic, blk.(*blockchain.ShardBlock)))
		}
		stateBytes, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		statePrefix := fmt.Sprintf("state-%v", shardID)
		if err := sim.userDB.Put([]byte(statePrefix), stateBytes, nil); err != nil {
			panic(err)
		}
		fmt.Printf("shard %v synced from %v to %v \n", shardID, state.LocalHeight, state.BestHeight)
		time.Sleep(5 * time.Second)
	}
}

func (sim *SimulationEngine) GetShardBlockByHeight(shardID byte, height uint64) *blockchain.ShardBlock {

}

func (sim *SimulationEngine) GetShardBlockByHash(shardID byte, blockHash []byte) *blockchain.ShardBlock {

}
