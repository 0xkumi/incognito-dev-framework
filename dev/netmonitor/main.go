package main

import (
	"fmt"
	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	network := devframework.NewNetworkMonitor("127.0.0.1:9330")
	network.OnReceive(devframework.MSG_BFT, func (msg interface{}){
		switch v := msg.(type) {
		default:
			fmt.Println("receive message", v)
		}
	})
	select {}
}
