package devframework

import (
	"os"
	"time"

	"github.com/0xkumi/incognito-dev-framework/account"
	"github.com/0xkumi/incognito-dev-framework/rpcclient"
)

func NewRPCClient(endpoint string) *rpcclient.RPCClient {
	return rpcclient.NewRPCClient(&RemoteRPCClient{endpoint: endpoint})
}

func NewStandaloneSimulation(name string, config Config) *NodeEngine {
	os.RemoveAll(name)
	// if config.ConsensusVersion < 1 || config.ConsensusVersion > 2 {
	if config.ConsensusVersion != 2 {
		config.ConsensusVersion = 2
	}
	sim := &NodeEngine{
		config:            config,
		simName:           name,
		timer:             NewTimeEngine(),
		accountSeed:       "master_account",
		accountGenHistory: make(map[int]int),
		committeeAccount:  make(map[int][]account.Account),
		listennerRegister: make(map[int][]func(msg interface{})),
	}
	sim.init()
	time.Sleep(1 * time.Second)
	return sim
}
