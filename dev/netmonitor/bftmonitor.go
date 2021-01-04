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
	return &bftmonitor{
		chainName,
		chainID,
	}, nil
}

func (s *bftmonitor) receiveBFTMsg(msg *wire.MessageBFT){
	fmt.Println("chain", s.chainID, "receive message", string(msg.Content), msg)
	//if msg.Type == "propose" {
	//
	//}
	//if msg.Type == "vote" {
	//
	//}
}




