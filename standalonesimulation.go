package devframework

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/consensustypes"
	"github.com/incognitochain/incognito-chain/consensus_v2/signatureschemes"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/multiview"
	"github.com/incognitochain/incognito-chain/portal"
	"github.com/incognitochain/incognito-chain/transaction/tx_ver2"

	"github.com/0xkumi/incognito-dev-framework/account"
	"github.com/0xkumi/incognito-dev-framework/mock"
	"github.com/0xkumi/incognito-dev-framework/rpcclient"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/incognitochain/incognito-chain/pubsub"

	"github.com/incognitochain/incognito-chain/syncker"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incdb"
	_ "github.com/incognitochain/incognito-chain/incdb/lvdb"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/rpcserver"

	lvdbErrors "github.com/syndtr/goleveldb/leveldb/errors"

	"github.com/pkg/errors"
)

type Config struct {
	ConsensusVersion int
	DisableLog       bool
}

type Hook struct {
	Create       func(chainID int, doCreate func() (blk types.BlockInterface, err error))
	Validation   func(chainID int, block types.BlockInterface, doValidation func(blk types.BlockInterface) error)
	CombineVotes func(chainID int) []int
	Insert       func(chainID int, block types.BlockInterface, doInsert func(blk types.BlockInterface) error)
}

type NodeEngine struct {
	config      Config
	appNodeMode string
	simName     string
	timer       *TimeEngine

	//for account manager
	accountSeed       string
	accountGenHistory map[int]int
	committeeAccount  map[int][]account.Account
	accounts          []*account.Account

	GenesisAccount account.Account

	//blockchain dependency object
	Network     *HighwayConnection
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

	userDB        *leveldb.DB
	lightNodeData struct {
		Shards                map[byte]*currentShardState
		ProcessedBeaconHeight uint64
	}
}

type currentShardState struct {
	// BestHeight  uint64
	// BestHash    *common.Hash
	LocalHeight uint64
	LocalHash   *common.Hash
}

func (sim *NodeEngine) NewAccountFromShard(sid int) account.Account {
	lastID := sim.accountGenHistory[sid]
	lastID++
	sim.accountGenHistory[sid] = lastID
	acc, _ := account.GenerateAccountByShard(sid, lastID, sim.accountSeed)
	acc.SetName(fmt.Sprintf("ACC_%v", len(sim.accounts)-len(sim.committeeAccount)+1))
	sim.accounts = append(sim.accounts, &acc)
	return acc
}

func (sim *NodeEngine) NewAccount() account.Account {
	lastID := sim.accountGenHistory[0]
	lastID++
	sim.accountGenHistory[0] = lastID
	acc, _ := account.GenerateAccountByShard(0, lastID, sim.accountSeed)
	return acc
}

func (sim *NodeEngine) init() {
	simName := sim.simName
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	disableStdoutLog = sim.config.DisableLog
	initLogRotator(filepath.Join(path, simName+".log"))
	dbLogger.SetLevel(common.LevelTrace)
	blockchainLogger.SetLevel(common.LevelTrace)
	bridgeLogger.SetLevel(common.LevelTrace)
	rpcLogger.SetLevel(common.LevelTrace)
	rpcServiceLogger.SetLevel(common.LevelTrace)
	rpcServiceBridgeLogger.SetLevel(common.LevelTrace)
	transactionLogger.SetLevel(common.LevelTrace)
	// privacyLogger.SetLevel(common.LevelTrace)
	mempoolLogger.SetLevel(common.LevelTrace)

	sim.appNodeMode = "full"
	cfg := config.LoadConfig()
	_ = cfg
	param := config.LoadParam()

	common.TIMESLOT = param.ConsensusParam.Timeslot
	common.MaxShardNumber = param.ActiveShards

	//load keys from file
	param.LoadKey(nil, nil)
	portal.SetupParam()

	sim.GenesisAccount = sim.NewAccount()
	for i := 0; i < param.CommitteeSize.MinBeaconCommitteeSize; i++ {
		acc := sim.NewAccountFromShard(-1)
		sim.committeeAccount[-1] = append(sim.committeeAccount[-1], acc)
		param.GenesisParam.PreSelectBeaconNodeSerializedPubkey = append(param.GenesisParam.PreSelectBeaconNodeSerializedPubkey, acc.SelfCommitteePubkey)
		param.GenesisParam.PreSelectBeaconNodeSerializedPaymentAddress = append(param.GenesisParam.PreSelectBeaconNodeSerializedPaymentAddress, acc.PaymentAddress)
	}
	for i := 0; i < param.ActiveShards; i++ {
		for a := 0; a < param.CommitteeSize.MinShardCommitteeSize; a++ {
			acc := sim.NewAccountFromShard(i)
			sim.committeeAccount[i] = append(sim.committeeAccount[i], acc)
			param.GenesisParam.PreSelectShardNodeSerializedPubkey = append(param.GenesisParam.PreSelectShardNodeSerializedPubkey, acc.SelfCommitteePubkey)
			param.GenesisParam.PreSelectShardNodeSerializedPaymentAddress = append(param.GenesisParam.PreSelectShardNodeSerializedPaymentAddress, acc.PaymentAddress)
		}
	}
	//TODO: can't add custom tx
	// initTxs := createGenesisTx([]account.Account{sim.GenesisAccount})
	// param.GenesisParam.InitialIncognito = initTxs

	//create genesis block
	blockchain.CreateGenesisBlocks()

	//init blockchain
	bc := blockchain.BlockChain{}

	layout := "2006-01-02T15:04:05.000Z"
	str := param.GenesisParam.BlockTimestamp
	genesisTime, err := time.Parse(layout, str)

	if err != nil {
		Logger.log.Error(err)
	}

	sim.timer.init(genesisTime.Unix() + 10)

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
	for i := byte(0); i < byte(param.ActiveShards); i++ {
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
	dbpath := filepath.Join("./"+simName, "database")
	db, err := incdb.OpenMultipleDB("leveldb", dbpath)
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
		BlockChain:     &bc,
		Blockgen:       blockgen,
		TxMemPool:      &txpool,
		Server:         &server,
		Database:       db,
	}
	rpcServer := &rpcserver.RpcServer{}
	rpclocal := &LocalRPCClient{rpcServer}

	btcChain, err := getBTCRelayingChain(portal.GetPortalParams().RelayingParam.BTCRelayingHeaderChainID, "btcchain", simName)
	if err != nil {
		panic(err)
	}
	bnbChainState, err := getBNBRelayingChainState(portal.GetPortalParams().RelayingParam.BNBRelayingHeaderChainID, simName)
	if err != nil {
		panic(err)
	}

	txpool.Init(&mempool.Config{
		ConsensusEngine: &cs,
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
	txpool.InitChannelMempool(cPendingTxs, cRemovedTxs)

	temppool.Init(&mempool.Config{
		BlockChain:    &bc,
		DataBase:      db,
		FeeEstimator:  fees,
		MaxTx:         1000,
		PubSubManager: ps,
	})
	txpool.IsBlockGenStarted = true
	go temppool.Start(cQuit)
	go txpool.Start(cQuit)

	var outcoinDb *incdb.Database = nil
	temp, err := incdb.Open("leveldb", filepath.Join("./"+simName, "outcoin"))
	if err != nil {
		Logger.log.Error("could not open leveldb instance for coin storing")
	}
	outcoinDb = &temp
	err = bc.Init(&blockchain.Config{
		BTCChain:      btcChain,
		BNBChainState: bnbChainState,
		DataBase:      db,
		MemCache:      memcache.New(),
		BlockGen:      blockgen,
		TxPool:        &txpool,
		TempTxPool:    &temppool,
		Server:        &server,
		Syncker:       sync,
		PubSubManager: ps,
		FeeEstimator:  make(map[byte]blockchain.FeeEstimator),
		// RandomClient:      &btcrd,
		ConsensusEngine:   &cs,
		OutCoinByOTAKeyDb: outcoinDb,
	})
	if err != nil {
		panic(err)
	}
	bc.InitChannelBlockchain(cRemovedTxs)

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

func (sim *NodeEngine) startPubSub() {
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

func (sim *NodeEngine) ConnectNetwork(highwayAddr string, relayShards []byte) {
	config := HighwayConnectionConfig{
		"127.0.0.1",
		19876,
		"2.0.0",
		highwayAddr,
		"",
		sim.consensus,
		sim.syncker,
		relayShards,
		// "",
	}
	sim.Network = NewHighwayConnection(config)
	sim.Network.Connect()

	sim.syncker.Init(&syncker.SynckerManagerConfig{Network: sim.Network.conn, Blockchain: sim.bc, Consensus: sim.consensus})
	sim.syncker.Start()

}

func (sim *NodeEngine) Pause() {
	log.Print("Simulation pause! Press Enter to continue ...")
	var input string
	fmt.Scanln(&input)
	log.Print("\n")
}

func (sim *NodeEngine) PrintBlockChainInfo() {
	log.Println("Beacon Chain:")

	log.Println("Shard Chain:")
}

//life cycle of a block generation process:
//PreCreate -> PreValidation -> PreInsert ->
func (sim *NodeEngine) GenerateBlock(args ...interface{}) *NodeEngine {
	time.Sleep(time.Nanosecond)
	var chainArray = []int{-1}
	for i := 0; i < config.Param().ActiveShards; i++ {
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
	var block types.BlockInterface = nil
	var err error

	//Create blocks for apply chain
	for _, chainID := range chainArray {
		var proposerPK incognitokey.CommitteePublicKey
		var bestview multiview.View
		if chainID == -1 {
			bestview = sim.bc.BeaconChain.GetBestView()
		} else {
			bestview = sim.bc.ShardChain[chainID].GetBestView()
		}
		switch sim.config.ConsensusVersion {
		case 1:
			var producerPosition int
			// if chainID == -1 {
			// 	lastProposerIdx := chain.BeaconChain.GetBestView().(*blockchain.BeaconBestState).BeaconProposerIndex
			// 	producerPosition = lastProposerIdx + 1%len(chain.BeaconChain.GetBestView().GetCommittee())
			// } else {
			// 	lastProposerIdx := chain.ShardChain[chainID].GetBestState().ShardProposerIdx
			// 	producerPosition = lastProposerIdx + 1%len(chain.GetChain(chainID).GetBestView().GetCommittee())
			// }
			proposerPK = bestview.GetCommittee()[producerPosition]
		case 2:
			proposerPK, _ = bestview.GetProposerByTimeSlot((sim.timer.Now() / 10), 2)
		}

		proposerPkStr, _ := proposerPK.ToBase58()
		if h != nil && h.Create != nil {
			h.Create(chainID, func() (blk types.BlockInterface, err error) {
				if chainID == -1 {
					block, err = chain.BeaconChain.CreateNewBlock(sim.config.ConsensusVersion, proposerPkStr, 1, sim.timer.Now(), []incognitokey.CommitteePublicKey{}, common.Hash{})
					if err != nil {
						block = nil
						return nil, err
					}
					return block, nil
				} else {
					block, err = chain.ShardChain[byte(chainID)].CreateNewBlock(sim.config.ConsensusVersion, proposerPkStr, 1, sim.timer.Now(), []incognitokey.CommitteePublicKey{}, common.Hash{})
					if err != nil {
						return nil, err
					}
					return block, nil
				}
			})
		} else {
			if chainID == -1 {
				block, err = chain.BeaconChain.CreateNewBlock(sim.config.ConsensusVersion, proposerPkStr, 1, sim.timer.Now(), []incognitokey.CommitteePublicKey{}, common.Hash{})
				if err != nil {
					block = nil
					log.Println("NewBlockError", err)
				}
			} else {
				block, err = chain.ShardChain[byte(chainID)].CreateNewBlock(sim.config.ConsensusVersion, proposerPkStr, 1, sim.timer.Now(), []incognitokey.CommitteePublicKey{}, common.Hash{})
				if err != nil {
					block = nil
					log.Println("NewBlockError", err)
				}
			}
		}

		//SignBlock
		proposeAcc := sim.GetAccountByCommitteePubkey(&proposerPK)
		userKey, _ := consensus_v2.GetMiningKeyFromPrivateSeed(proposeAcc.MiningKey)
		sim.SignBlock(userKey, block)

		//Validation
		if h != nil && h.Validation != nil {
			h.Validation(chainID, block, func(blk types.BlockInterface) (err error) {
				if blk == nil {
					return errors.New("No block for validation")
				}
				if chainID == -1 {
					err = chain.VerifyPreSignBeaconBlock(blk.(*types.BeaconBlock), true)
					if err != nil {
						return err
					}
					return nil
				} else {
					err = chain.VerifyPreSignShardBlock(blk.(*types.ShardBlock), chain.ShardChain[chainID].GetBestState().GetShardCommittee(), byte(chainID))
					if err != nil {
						return err
					}
					return nil
				}
			})
		} else {
			if block == nil {
				log.Println("VerifyBlockErr no block")
			} else {
				if chainID == -1 {
					err = chain.VerifyPreSignBeaconBlock(block.(*types.BeaconBlock), true)
					if err != nil {
						log.Println("VerifyBlockErr", err)
					}
				} else {
					err = chain.VerifyPreSignShardBlock(block.(*types.ShardBlock), chain.ShardChain[chainID].GetBestState().GetShardCommittee(), byte(chainID))
					if err != nil {
						log.Println("VerifyBlockErr", err)
					}
				}
			}

		}

		//Combine votes
		accs, err := sim.GetListAccountsByChainID(chainID)
		if err != nil {
			panic(err)
		}
		if h != nil && h.CombineVotes != nil {
			nCombine := h.CombineVotes(chainID)
			if nCombine == nil {
				nCombine = GenerateCommitteeIndex(len(bestview.GetCommittee()))
			}
			sim.SignBlockWithCommittee(block, accs, nCombine)
		} else {
			sim.SignBlockWithCommittee(block, accs, GenerateCommitteeIndex(len(bestview.GetCommittee())))
		}

		//Insert
		if h != nil && h.Insert != nil {
			h.Insert(chainID, block, func(blk types.BlockInterface) (err error) {
				if blk == nil {
					return errors.New("No block for insert")
				}
				if chainID == -1 {
					err = chain.InsertBeaconBlock(blk.(*types.BeaconBlock), true)
					if err != nil {
						return err
					}
					return
				} else {
					err = chain.InsertShardBlock(blk.(*types.ShardBlock), true)
					if err != nil {
						return err
					} else {
						crossX := types.CreateAllCrossShardBlock(block.(*types.ShardBlock), config.Param().ActiveShards)
						for _, blk := range crossX {
							sim.syncker.InsertCrossShardBlock(blk)
						}
					}
					return
				}
			})
		} else {
			if block == nil {
				log.Println("InsertBlkErr no block")
			} else {
				if chainID == -1 {
					err = chain.InsertBeaconBlock(block.(*types.BeaconBlock), true)
					if err != nil {
						log.Println("InsertBlkErr", err)
					}
				} else {
					err = chain.InsertShardBlock(block.(*types.ShardBlock), true)
					if err != nil {
						log.Println("InsertBlkErr", err)
					} else {
						crossX := types.CreateAllCrossShardBlock(block.(*types.ShardBlock), config.Param().ActiveShards)
						for _, blk := range crossX {
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
func (sim *NodeEngine) NextRound() {
	sim.timer.Forward(10)
}

func (sim *NodeEngine) InjectTx(txBase58 string, isTxToken bool) error {
	rawTxBytes, _, err := base58.Base58Check{}.Decode(txBase58)
	if err != nil {
		return err
	}
	if isTxToken {
		var tx tx_ver2.TxToken
		err = json.Unmarshal(rawTxBytes, &tx)
		if err != nil {
			return err
		}
		sim.cPendingTxs <- &tx

	} else {
		var tx tx_ver2.Tx
		err = json.Unmarshal(rawTxBytes, &tx)
		if err != nil {
			return err
		}
		sim.cPendingTxs <- &tx

	}
	return nil
}

func (sim *NodeEngine) GetBlockchain() *blockchain.BlockChain {
	return sim.bc
}

func (s *NodeEngine) GetUserDatabase() *leveldb.DB {
	return s.userDB
}

func (s *NodeEngine) DisableChainLog(b bool) {
	disableStdoutLog = b
}

func (s *NodeEngine) SignBlockWithCommittee(block types.BlockInterface, committees []account.Account, committeeIndex []int) error {
	// committeePubKey := []incognitokey.CommitteePublicKey{}
	// miningKeys := []*signatureschemes.MiningKey{}
	if block.GetVersion() == 2 {
		// votes := make(map[string]*blsbft.BFTVote)
		// for _, committee := range committees {
		// 	miningKey, _ := consensus_v2.GetMiningKeyFromPrivateSeed(committee.MiningKey)
		// 	committeePubKey = append(committeePubKey, *miningKey.GetPublicKey())
		// 	miningKeys = append(miningKeys, miningKey)
		// }
		// for _, committeeID := range committeeIndex {
		// 	vote, _ := blsbft.CreateVote(miningKeys[committeeID], block, committeePubKey, s.bc.BeaconChain.GetPortalParamsV4(0))
		// 	vote.IsValid = 1
		// 	votes[vote.Validator] = vote
		// }
		// committeeBLSString, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(committeePubKey, common.BlsConsensus)
		// aggSig, brigSigs, validatorIdx, portalSigs, err := blsbft.CombineVotes(votes, committeeBLSString)

		// valData, err := consensustypes.DecodeValidationData(block.GetValidationField())
		// if err != nil {
		// 	return errors.New("decode validation data")
		// }
		// valData.AggSig = aggSig
		// valData.BridgeSig = brigSigs
		// valData.ValidatiorsIdx = validatorIdx
		// valData.PortalSig = portalSigs
		// validationDataString, _ := consensustypes.EncodeValidationData(*valData)
		// if err := block.(mock.BlockValidation).AddValidationField(validationDataString); err != nil {
		// 	return errors.New("Add validation error")
		// }
	}
	return nil
}

func (s *NodeEngine) SignBlock(userMiningKey *signatureschemes.MiningKey, block types.BlockInterface) {
	var validationData consensustypes.ValidationData
	validationData.ProducerBLSSig, _ = userMiningKey.BriSignData(block.Hash().GetBytes())
	validationDataString, _ := consensustypes.EncodeValidationData(validationData)
	block.(mock.BlockValidation).AddValidationField(validationDataString)
}

func (s *NodeEngine) GetAccountByCommitteePubkey(cpk *incognitokey.CommitteePublicKey) *account.Account {
	miningPK := cpk.GetMiningKeyBase58(common.BlsConsensus)
	for _, acc := range s.accounts {
		if acc.MiningPubkey == miningPK {
			return acc
		}
	}
	return nil
}

func (s *NodeEngine) GetListAccountByCommitteePubkey(cpks []incognitokey.CommitteePublicKey) ([]account.Account, error) {
	accounts := []account.Account{}
	for _, cpk := range cpks {
		if acc := s.GetAccountByCommitteePubkey(&cpk); acc != nil {
			accounts = append(accounts, *acc)
		}
	}
	if len(accounts) != len(cpks) {
		return nil, errors.New("Mismatch number of committee pubkey in beststate")
	}
	return accounts, nil
}

func (sim *NodeEngine) GetListAccountsByChainID(chainID int) ([]account.Account, error) {
	var committees []incognitokey.CommitteePublicKey
	if chainID == -1 {
		committees = sim.bc.BeaconChain.GetBestView().GetCommittee()
	} else {
		committees = sim.bc.ShardChain[chainID].GetBestView().GetCommittee()
	}
	return sim.GetListAccountByCommitteePubkey(committees)
}
