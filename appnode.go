package devframework

import (
	"net"
	"path/filepath"

	"github.com/0xkumi/incongito-dev-framework/mock"
	"github.com/0xkumi/incongito-dev-framework/rpcclient"

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

const (
	MODE_MAINNET = iota
	MODE_TESTNET
	MODE_TESTNET2
	MODE_CUSTOM
)

type AppNodeInterface interface {
	OnReceive(msgType int, f func(msg interface{}))
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
	DisableChainLog(disable bool)
	GetBlockchain() *blockchain.BlockChain
	GetRPC() *rpcclient.RPCClient
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

func NewAppNode(name string, networkParam NetworkParam, enableRPC bool) AppNodeInterface {
	// os.RemoveAll(name)
	sim := &SimulationEngine{
		simName: name,
	}
	chainParam := &blockchain.Params{}
	switch networkParam {
	case MainNetParam:
		blockchain.IsTestNet = false
		blockchain.IsTestNet2 = false
		blockchain.ReadKey(blockchain.MainnetKeylist, blockchain.Mainnetv2Keylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainMainParam
		break
	case TestNetParam:
		blockchain.IsTestNet = true
		blockchain.IsTestNet2 = false
		blockchain.ReadKey(blockchain.TestnetKeylist, blockchain.Testnetv2Keylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainTestParam
		break
	case TestNet2Param:
		blockchain.IsTestNet = true
		blockchain.IsTestNet2 = true
		blockchain.ReadKey(blockchain.Testnet2Keylist, blockchain.Testnet2v2Keylist)
		blockchain.SetupParam()
		chainParam = &blockchain.ChainTest2Param
		break
	default:
		blockchain.IsTestNet = false
		blockchain.IsTestNet2 = false
		break
	}
	sim.initNode(chainParam, enableRPC)
	relayShards := []byte{}
	for index := 0; index < common.MaxShardNumber; index++ {
		relayShards = append(relayShards, byte(index))
	}
	sim.ConnectNetwork(networkParam.HighwayAddress, relayShards)
	return sim
}

func (sim *SimulationEngine) initNode(chainParam *blockchain.Params, enableRPC bool) {
	simName := sim.simName
	//path, err := os.Getwd()
	//if err != nil {
	//	log.Println(err)
	//}
	//initLogRotator(filepath.Join(path, simName+".log"))
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

	db, err := incdb.OpenMultipleDB("leveldb", filepath.Join("./"+simName, "database"))
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
}

func (sim *SimulationEngine) GetRPC() *rpcclient.RPCClient {
	return sim.RPC
}
