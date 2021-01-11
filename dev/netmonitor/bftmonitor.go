package main

import (
	"fmt"
	"github.com/incognitochain/incognito-chain/wire"

	"regexp"
	"strconv"
)


//this monitor
//bft block fork
//bft timing

type bftmonitor struct {
	chainName string
	chainID int
	dbConnection BFTMessageDB
}

func NewBFTMonitor(chainName string) (b *bftmonitor,err error) {
	chainID := -1
	if chainName != "beacon" {
		regex,_ := regexp.Compile(`shard-(\d)`)
		str := regex.FindStringSubmatch(chainName)
		chainID, err = strconv.Atoi(str[1])
		if err != nil {
			return nil, err
		}
	}
	dbConn,err:= NewBFTMessageCollection("mongodb://localhost:38118", "netmonitor", "bft")

	if err != nil {
		fmt.Println(err)
	}

	return &bftmonitor{
		chainName,
		chainID,
		dbConn,

	}, nil
}

func (s *bftmonitor) receiveBFTMsg(msg *wire.MessageBFT){
	//fmt.Println("chain", s.chainID, "receive message", string(msg.Content))
	s.dbConnection.Save(msg)
}




