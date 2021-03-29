package devframework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	bnbrelaying "github.com/incognitochain/incognito-chain/relaying/bnb"
	btcrelaying "github.com/incognitochain/incognito-chain/relaying/btc"
	"github.com/incognitochain/incognito-chain/transaction"

	"github.com/0xkumi/incognito-dev-framework/account"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/wallet"
)

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

func createGenesisTx(accounts []account.Account) []string {
	transactions := []string{}
	db, err := incdb.Open("leveldb", "/tmp/"+time.Now().UTC().String())
	if err != nil {
		log.Print("could not open connection to leveldb")
		log.Print(err)
		panic(err)
	}
	stateDB, err := statedb.NewWithPrefixTrie(common.EmptyRoot, statedb.NewDatabaseAccessWarper(db))
	if err != nil {
		panic(err)
	}
	initPRV := int(1000000000000 * math.Pow(10, 9))
	for _, account := range accounts {
		txs := initSalaryTx(strconv.Itoa(initPRV), account.PrivateKey, stateDB)
		transactions = append(transactions, txs[0])
	}
	return transactions
}

func initSalaryTx(amount string, privateKey string, stateDB *statedb.StateDB) []string {
	var initTxs []string
	var initAmount, _ = strconv.Atoi(amount) // amount init
	testUserkeyList := []string{
		privateKey,
	}
	for _, val := range testUserkeyList {

		testUserKey, _ := wallet.Base58CheckDeserialize(val)
		testUserKey.KeySet.InitFromPrivateKey(&testUserKey.KeySet.PrivateKey)
		otaCoin, err := coin.NewCoinFromAmountAndReceiver(uint64(initAmount), testUserKey.KeySet.PaymentAddress)
		if err != nil {
			panic(err)
		}
		// testSalaryTX := tx_ver1.Tx{}
		// testSalaryTX.InitTxSalary(uint64(initAmount), &testUserKey.KeySet.PaymentAddress, &testUserKey.KeySet.PrivateKey,
		// 	stateDB,
		// 	nil,
		// )
		testSalaryTX := new(transaction.TxVersion2)
		testSalaryTX.InitTxSalary(otaCoin, &testUserKey.KeySet.PrivateKey, stateDB, nil)

		initTx, _ := json.Marshal(testSalaryTX)
		initTxs = append(initTxs, string(initTx))
	}
	return initTxs
}
func DownloadLatestBackup(remoteURL string, chainID int) error {
	chainName := "beacon"
	if chainID > -1 {
		chainName = fmt.Sprintf("shard%v", chainID)
	}
	backupFile := "./data/preload/" + chainName
	fd, err := os.OpenFile(backupFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	fd.Truncate(0)
	err = makeRPCDownloadRequest(remoteURL, "downloadbackup", fd, chainName)
	if err != nil {
		return err
	}
	fd.Close()
	if chainName == "beacon" {
		fd, err = os.OpenFile("./data/preload/btc", os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		fd.Truncate(0)
		err = makeRPCDownloadRequest(remoteURL, "downloadbackup", fd, chainName, "btc")
		if err != nil {
			return err
		}
		fd.Close()
	}

	log.Println("Download finish", chainName)
	return nil
}

type JsonRequest struct {
	Jsonrpc string      `json:"Jsonrpc"`
	Method  string      `json:"Method"`
	Params  interface{} `json:"Params"`
	Id      interface{} `json:"Id"`
}

func makeRPCDownloadRequest(address string, method string, w io.Writer, params ...interface{}) error {
	request := JsonRequest{
		Jsonrpc: "1.0",
		Method:  method,
		Params:  params,
		Id:      "1",
	}
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		return err
	}
	log.Println(string(requestBytes))
	resp, err := http.Post(address, "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		log.Println(err)
		return err
	}
	n, err := io.Copy(w, resp.Body)
	log.Println(n, err)
	if err != nil {
		return err
	}
	return nil
}

func (sim *NodeEngine) SendPRV(args ...interface{}) (string, error) {
	var sender string
	var receivers = make(map[string]uint64)
	for i, arg := range args {
		if i == 0 {
			sender = arg.(account.Account).PrivateKey
		} else {
			switch arg.(type) {
			default:
				if i%2 == 1 {
					amount, ok := args[i+1].(int)
					if !ok {
						amountF64 := args[i+1].(int)
						amount = amountF64
					}
					receivers[arg.(account.Account).PaymentAddress] = uint64(amount)
				}
			}
		}
	}
	res, err := sim.RPC.API_SendTxPRV(sender, receivers, -1, true)
	if err != nil {
		log.Println(err)
		sim.Pause()
	}
	return res.TxID, nil
}

func (sim *NodeEngine) ShowBalance(acc account.Account) {
	res, err := sim.RPC.API_GetBalance(acc)
	if err != nil {
		log.Println(err)
	}
	log.Println(res)
}

func GenerateCommitteeIndex(nCommittee int) []int {
	res := []int{}
	for i := 0; i < nCommittee; i++ {
		res = append(res, i)
	}
	return res
}
