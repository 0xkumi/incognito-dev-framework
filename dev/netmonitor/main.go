package main

import (
	"fmt"
	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/incognito-chain/wire"
)

var BFTMonitorProcesses = make(map[string]*bftmonitor)

func main() {
	network := devframework.NewNetworkMonitor("127.0.0.1:9330")

	network.OnReceive(devframework.MSG_BFT, func (msg interface{}){
		switch v := msg.(type) {
		case *wire.MessageBFT:
			if _,ok := BFTMonitorProcesses[v.ChainKey]; !ok {
				b ,err := NewBFTMonitor(v.ChainKey)
				BFTMonitorProcesses[v.ChainKey] = b
				//fmt.Println("set bft", b, err)
				if err != nil {
					fmt.Println(err)
					return
				}
			}
			BFTMonitorProcesses[v.ChainKey].receiveBFTMsg(v)
		default:
			fmt.Println("receive message", v, msg)
		}
	})

	network.OnReceive(devframework.MSG_PEER_STATE, func (msg interface{}){
		switch v := msg.(type) {
		case *wire.MessagePeerState:
			//update 1 record per peerstate
		default:
			fmt.Println("receive message", v, msg)
		}
	})
	select {}
}
