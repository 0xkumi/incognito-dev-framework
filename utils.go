package devframework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/0xkumi/incongito-dev-framework/account"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
)

func createGenesisTx(accounts []account.Account) []string {
	transactions := []string{}
	db, err := incdb.Open("leveldb", "/tmp/"+time.Now().UTC().String())
	if err != nil {
		fmt.Print("could not open connection to leveldb")
		fmt.Print(err)
		panic(err)
	}
	stateDB, _ := statedb.NewWithPrefixTrie(common.EmptyRoot, statedb.NewDatabaseAccessWarper(db))
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
		testSalaryTX := transaction.Tx{}
		testSalaryTX.InitTxSalary(uint64(initAmount), &testUserKey.KeySet.PaymentAddress, &testUserKey.KeySet.PrivateKey,
			stateDB,
			nil,
		)
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

	fmt.Println("Download finish", chainName)
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
	fmt.Println(string(requestBytes))
	resp, err := http.Post(address, "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		fmt.Println(err)
		return err
	}
	n, err := io.Copy(w, resp.Body)
	fmt.Println(n, err)
	if err != nil {
		return err
	}
	return nil
}

func (sim *SimulationEngine) SendPRV(args ...interface{}) (string, error) {
	var sender string
	var receivers = make(map[string]interface{})
	for i, arg := range args {
		if i == 0 {
			sender = arg.(account.Account).PrivateKey
		} else {
			switch arg.(type) {
			default:
				if i%2 == 1 {
					amount, ok := args[i+1].(int)
					if !ok {
						amountF64 := args[i+1].(float64)
						amount = int(amountF64)
					}
					receivers[arg.(account.Account).PaymentAddress] = float64(amount)
				}
			}
		}
	}

	res, err := sim.RPC.API_SendTxPRV(sender, receivers, 1, 1)
	if err != nil {
		fmt.Println(err)
		sim.Pause()
	}
	return res.TxID, nil
}

func (sim *SimulationEngine) ShowBalance(acc account.Account)  {
	res,err := sim.RPC.API_GetBalance(acc)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)
}