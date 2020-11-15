package devframework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

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
