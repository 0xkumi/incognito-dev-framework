package devframework

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/0xkumi/incognito-dev-framework/mock"
	"github.com/0xkumi/incognito-dev-framework/rpcclient"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	lvdbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/portal"
	"github.com/incognitochain/incognito-chain/pubsub"
	"github.com/incognitochain/incognito-chain/rpcserver"
	"github.com/incognitochain/incognito-chain/syncker"
	"github.com/incognitochain/incognito-chain/txpool"
)

type AppNodeInterface interface {
	OnReceive(msgType int, f func(msg interface{}))
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
	OnInserted(blkType int, f func(msg interface{}))
	DisableChainLog(bool)
	GetBlockchain() *blockchain.BlockChain
	GetRPC() *rpcclient.RPCClient
	GetUserDatabase() *leveldb.DB
	GetShardState(shardID int) (uint64, *common.Hash)
	// LightNode() LightNodeInterface
}

type LightNodeInterface interface {
}
type NetworkParam struct {
	Name           string
	ChainParam     interface{}
	HighwayAddress string
}

var (
	MainNetParam = NetworkParam{
		Name:           "mainnet",
		HighwayAddress: "51.83.237.29:9330",
	}
	TestNetParam = NetworkParam{
		Name:           "testnet",
		HighwayAddress: "45.56.115.6:9330",
	}
	TestNet2Param = NetworkParam{
		Name:           "testnet2",
		HighwayAddress: "74.207.247.250:9330",
	}
)

func NewNetworkMonitor(highwayAddr string) *HighwayConnection {
	config := HighwayConnectionConfig{
		"127.0.0.1",
		19876,
		"2.0.0",
		highwayAddr,
		"",
		nil,
		nil,
		nil,
		// "netmonitor",
	}
	network := NewHighwayConnection(config)
	network.Connect()
	return network
}

func NewAppNode(name string, networkParam NetworkParam, isLightNode bool, requireFinalizedBeacon bool, enableRPC bool, disableLogFile bool) AppNodeInterface {
	// os.RemoveAll(name)
	nodeMode := "full"
	if isLightNode {
		nodeMode = "light"
	}
	sim := &NodeEngine{
		simName:           name,
		appNodeMode:       nodeMode,
		listennerRegister: make(map[int][]func(msg interface{})),
	}
	cfg := config.LoadConfig()
	_ = cfg
	param := config.LoadParam()

	common.TIMESLOT = param.ConsensusParam.Timeslot
	common.MaxShardNumber = param.ActiveShards

	//load keys from file
	param.LoadKey()
	portal.SetupParam()

	//create genesis block
	blockchain.CreateGenesisBlocks()

	if common.TIMESLOT == 0 {
		common.TIMESLOT = 10
	}
	sim.initNode(isLightNode, enableRPC, disableLogFile)
	relayShards := []byte{}
	if isLightNode {
		for index := 0; index < common.MaxShardNumber; index++ {
			relayShards = append(relayShards, byte(index))
		}
		go sim.startLightSyncProcess(requireFinalizedBeacon)
	} else {
		for index := 0; index < common.MaxShardNumber; index++ {
			relayShards = append(relayShards, byte(index))
		}
	}
	sim.ConnectNetwork(networkParam.HighwayAddress, relayShards)
	sim.DisableChainLog(true)

	return sim
}

func (sim *NodeEngine) initNode(isLightNode bool, enableRPC bool, disableLogFile bool) {
	simName := sim.simName
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	if !disableLogFile {
		initLogRotator(filepath.Join(path, simName+".log"))
	}
	dbLogger.SetLevel(common.LevelTrace)
	blockchainLogger.SetLevel(common.LevelTrace)
	bridgeLogger.SetLevel(common.LevelTrace)
	rpcLogger.SetLevel(common.LevelTrace)
	rpcServiceLogger.SetLevel(common.LevelTrace)
	rpcServiceBridgeLogger.SetLevel(common.LevelTrace)
	transactionLogger.SetLevel(common.LevelTrace)
	// privacyLogger.SetLevel(common.LevelTrace)
	mempoolLogger.SetLevel(common.LevelTrace)
	activeNetParams := config.Param()

	common.MaxShardNumber = activeNetParams.ActiveShards
	//init blockchain
	bc := blockchain.BlockChain{}

	cs := consensus_v2.NewConsensusEngine()
	txPool := mempool.TxPool{}
	tempPool := mempool.TxPool{}
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
	blockgen, err := blockchain.NewBlockGenerator(&txPool, &bc, sync, cPendingTxs, cRemovedTxs)
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
		BlockChain:     &bc,
		Blockgen:       blockgen,
		TxMemPool:      &txPool,
		Server:         &server,
		Database:       db,
	}
	rpcServer := &rpcserver.RpcServer{}
	rpclocal := &LocalRPCClient{rpcServer}

	btcChain, err := getBTCRelayingChain(portal.GetPortalParams().RelayingParam.BTCRelayingHeaderChainID, portal.GetPortalParams().RelayingParam.BTCDataFolderName, simName)
	if err != nil {
		panic(err)
	}
	bnbChainState, err := getBNBRelayingChainState(portal.GetPortalParams().RelayingParam.BNBRelayingHeaderChainID, simName)
	if err != nil {
		panic(err)
	}
	txPool.Init(&mempool.Config{
		ConsensusEngine: cs,
		BlockChain:      &bc,
		DataBase:        db,
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
	txPool.InitChannelMempool(cPendingTxs, cRemovedTxs)

	tempPool.Init(&mempool.Config{
		BlockChain:    &bc,
		DataBase:      db,
		FeeEstimator:  fees,
		MaxTx:         1000,
		PubSubManager: ps,
	})
	txPool.IsBlockGenStarted = true
	go tempPool.Start(cQuit)
	go txPool.Start(cQuit)

	relayShards := []byte{}
	if !isLightNode {
		for index := 0; index < common.MaxShardNumber; index++ {
			relayShards = append(relayShards, byte(index))
		}
	}
	poolManager, _ := txpool.NewPoolManager(
		common.MaxShardNumber,
		ps,
		time.Duration(2)*time.Second,
	)

	err = bc.Init(&blockchain.Config{
		BTCChain:      btcChain,
		BNBChainState: bnbChainState,
		DataBase:      db,
		MemCache:      memcache.New(),
		BlockGen:      blockgen,
		TxPool:        &txPool,
		TempTxPool:    &tempPool,
		Server:        &server,
		Syncker:       sync,
		PubSubManager: ps,
		FeeEstimator:  make(map[byte]blockchain.FeeEstimator),
		// RandomClient:    &btcrd,
		ConsensusEngine: cs,
		RelayShards:     relayShards,
		PoolManager:     poolManager,
	})
	if err != nil {
		panic(err)
	}
	bc.InitChannelBlockchain(cRemovedTxs)

	sim.bc = &bc
	sim.consensus = cs
	sim.txpool = &txPool
	sim.temppool = &tempPool
	sim.btcrd = &btcrd
	sim.syncker = sync
	sim.server = &server
	sim.cPendingTxs = cPendingTxs
	sim.cRemovedTxs = cRemovedTxs
	sim.rpcServer = rpcServer
	sim.RPC = rpcclient.NewRPCClient(rpclocal)
	sim.cQuit = cQuit
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
	if enableRPC {
		go rpcServer.Start()
	}
	sim.startPubSub()
	//init syncker
	sim.syncker.Init(&syncker.SynckerManagerConfig{Blockchain: sim.bc})

	//init user database
	handles := -1
	cache := 8
	userDBPath := filepath.Join(dbpath, "userdb")
	lvdb, err := leveldb.OpenFile(userDBPath, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache * opt.MiB, // Two of these are used internally
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

func (sim *NodeEngine) GetRPC() *rpcclient.RPCClient {
	return sim.RPC
}

func (sim *NodeEngine) startLightSyncProcess(requireFinalizedBeacon bool) {
	log.Println("start light sync process")
	sim.lightNodeData.ProcessedBeaconHeight = 1
	k1 := "lightn-beacon-process"
	v1, err := sim.userDB.Get([]byte(k1), nil)
	if err != nil && err != leveldb.ErrNotFound {
		panic(err)
	}
	if err == nil {
		sim.lightNodeData.ProcessedBeaconHeight = binary.LittleEndian.Uint64(v1)
	}
	processBeaconBlk := func(bc *blockchain.BlockChain, h common.Hash, height uint64) {
		for i := sim.lightNodeData.ProcessedBeaconHeight; i <= height; i++ {
		retry:
			blks, err := sim.bc.GetBeaconBlockByHeight(i)
			if err != nil {
				log.Println(err)
				goto retry
			}
			beaconBlk := blks[0]
			if beaconBlk.Header.Height == 1 {
				for i := 0; i < config.Param().ActiveShards; i++ {
					blk, err := sim.bc.GetShardBlockByHeightV1(1, byte(i))
					if err != nil {
						panic(err)
					}
					key := fmt.Sprintf("s-%v-%v", byte(i), blk.GetHeight())
					if err := sim.userDB.Put([]byte(key), blk.Hash().Bytes(), nil); err != nil {
						panic(err)
					}
				}
			} else {
				for shardID, states := range beaconBlk.Body.ShardState {
					go func(sID byte, sts []types.ShardState) {
						for _, blk := range sts {
							key := fmt.Sprintf("s-%v-%v", sID, blk.Height)
							if err := sim.userDB.Put([]byte(key), blk.Hash.Bytes(), nil); err != nil {
								panic(err)
							}
						}
					}(shardID, states)
				}
			}
			sim.lightNodeData.ProcessedBeaconHeight = height
			key := "lightn-beacon-process"
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(sim.lightNodeData.ProcessedBeaconHeight))
			err = sim.userDB.Put([]byte(key), b, nil)
			if err != nil {
				panic(err)
			}
		}

	}
	sim.OnNewBlockFromParticularHeight(-1, int64(sim.lightNodeData.ProcessedBeaconHeight), requireFinalizedBeacon, processBeaconBlk)

	sim.lightNodeData.Shards = make(map[byte]*currentShardState)

	sim.loadLightShardsState()

	time.Sleep(2 * time.Second)
	for i := 0; i < config.Param().ActiveShards; i++ {
		go sim.syncShardLight(byte(i), sim.lightNodeData.Shards[byte(i)])
	}
}

func (sim *NodeEngine) loadLightShardsState() {
	for i := 0; i < config.Param().ActiveShards; i++ {
		statePrefix := fmt.Sprintf("state-%v", i)
		v, err := sim.userDB.Get([]byte(statePrefix), nil)
		if err != nil && err != leveldb.ErrNotFound {
			panic(err)
		}
		if err == leveldb.ErrNotFound {
			hash := sim.bc.ShardChain[i].GetBestState().BestBlock.Hash()
			height := sim.bc.ShardChain[i].GetBestState().BestBlock.GetHeight()
			sim.lightNodeData.Shards[byte(i)] = &currentShardState{
				LocalHeight: height,
				LocalHash:   hash,
			}
			key := fmt.Sprintf("s-%v-%v", i, height)
			if err := sim.userDB.Put([]byte(key), hash.Bytes(), nil); err != nil {
				panic(err)
			}
			continue
		}
		shardState := &currentShardState{}
		if err := json.Unmarshal(v, shardState); err != nil {
			panic(err)
		}
		sim.lightNodeData.Shards[byte(i)] = shardState
	}
}

func (sim *NodeEngine) GetShardState(shardID int) (uint64, *common.Hash) {
	statePrefix := fmt.Sprintf("state-%v", shardID)
	v, err := sim.userDB.Get([]byte(statePrefix), nil)
	if err != nil && err != leveldb.ErrNotFound {
		return 0, nil
	}
	shardState := &currentShardState{}
	if err := json.Unmarshal(v, shardState); err != nil {
		return 0, nil
	}

	return shardState.LocalHeight, shardState.LocalHash
}

func (sim *NodeEngine) syncShardLight(shardID byte, state *currentShardState) {
	log.Println("start sync shard", shardID, state.LocalHeight)
	blk, err := sim.bc.GetShardBlockByHeightV1(1, shardID)
	if err != nil {
		panic(err)
	}

	blkBytes, err := json.Marshal(blk)
	if err != nil {
		panic(err)
	}
	blkHash := blk.Hash()
	if err := sim.userDB.Put(blkHash.Bytes(), blkBytes, nil); err != nil {
		panic(err)
	}

	if state.LocalHeight == 1 {
		state.LocalHeight = blk.GetHeight()
		state.LocalHash = blkHash
		stateBytes, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		statePrefix := fmt.Sprintf("state-%v", shardID)
		if err := sim.userDB.Put([]byte(statePrefix), stateBytes, nil); err != nil {
			panic(err)
		}
	}
	for {
		bestHeight := sim.bc.BeaconChain.GetShardBestViewHeight()[shardID]
		// bestHash := sim.bc.BeaconChain.GetShardBestViewHash()[shardID]
		// state.BestHeight = bestHeight
		// state.BestHash = &bestHash
		blkCh, err := sim.Network.GetShardBlock(int(shardID), state.LocalHeight, bestHeight+1)
		fmt.Printf("Request shard %v block from %v to %v\n", shardID, state.LocalHeight, bestHeight+1)
		if err != nil && err.Error() != "requester not ready" {
			panic(err)
		}
		if err != nil && err.Error() == "requester not ready" {
			time.Sleep(5 * time.Second)
			continue
		}
		for {
			blk := <-blkCh
			if !isNil(blk) {
				blkBytes, err := json.Marshal(blk)
				if err != nil {
					panic(err)
				}

				blkHash := blk.(*types.ShardBlock).Hash()
				key := fmt.Sprintf("s-%v-%v", shardID, blk.GetHeight())
				blkHashBytes, err := sim.userDB.Get([]byte(key), nil)
				if err != nil {
					if err.Error() == "leveldb: not found" {
						log.Println(err)
						time.Sleep(2 * time.Second)
						break
					}
					panic(err)
				}
				blkLocalHash, err := common.Hash{}.NewHash(blkHashBytes)
				if err != nil {
					panic(err)
				}
				if !blkHash.IsEqual(blkLocalHash) {
					fmt.Printf("wrong block hash need %v get %v\n", blkLocalHash.String(), blkHash.String())
					panic(errors.New("Synced block has wrong block hash 🙀"))
				}
				if err := sim.userDB.Put(blkHash.Bytes(), blkBytes, nil); err != nil {
					panic(err)
				}
				state.LocalHeight = blk.GetHeight()
				state.LocalHash = blkHash
				stateBytes, err := json.Marshal(state)
				if err != nil {
					panic(err)
				}
				statePrefix := fmt.Sprintf("state-%v", shardID)
				if err := sim.userDB.Put([]byte(statePrefix), stateBytes, nil); err != nil {
					panic(err)
				}
			} else {
				break
			}
		}

		if state.LocalHeight == bestHeight {
			time.Sleep(1 * time.Second)
		}
		log.Printf("shard %v synced to %v at beacon height %v \n", shardID, state.LocalHeight, sim.bc.BeaconChain.CurrentHeight())

	}
}

func (sim *NodeEngine) GetShardBlockByHeight(shardID byte, height uint64) (*types.ShardBlock, error) {
	var shardBlk *types.ShardBlock
	if sim.appNodeMode == "light" {
		prefix := fmt.Sprintf("s-%v-%v", shardID, height)
		blkHash, err := sim.userDB.Get([]byte(prefix), nil)
		if err != nil {
			return nil, err
		}
		blkBytes, err := sim.userDB.Get(blkHash, nil)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(blkBytes, shardBlk); err != nil {
			return nil, err
		}
	} else {

	}
	return shardBlk, nil
}

func (sim *NodeEngine) GetShardBlockByHash(shardID byte, blockHash common.Hash) (*types.ShardBlock, error) {
	var shardBlk *types.ShardBlock
	var err error
	if sim.appNodeMode == "light" {
		blkBytes, err := sim.userDB.Get(blockHash.Bytes(), nil)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(blkBytes, shardBlk); err != nil {
			return nil, err
		}
	} else {
		shardBlk, _, err = sim.GetBlockchain().GetShardBlockByHashWithShardID(blockHash, shardID)
		if err != nil {
			return nil, err
		}
	}
	return shardBlk, nil
}

func (sim *NodeEngine) GetBeaconBlockByHeight(height uint64) (*types.BeaconBlock, error) {
	blks, err := sim.GetBlockchain().GetBeaconBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return blks[0], nil
}

func (sim *NodeEngine) GetBeaconBlockByHash(blockHash common.Hash) (*types.BeaconBlock, error) {
	blk, _, err := sim.GetBlockchain().GetBeaconBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	return blk, nil
}

func isNil(v interface{}) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}
