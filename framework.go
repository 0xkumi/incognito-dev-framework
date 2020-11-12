package devframework

import (
	"incognito-dev-framework/account"
	"incognito-dev-framework/rpcclient"
	"os"
	"time"

)

func NewRPCClient(endpoint string) *rpcclient.RPCClient {
	return rpcclient.NewRPCClient(&RemoteRPCClient{endpoint: endpoint})
}

func NewStandaloneSimulation(name string, config Config) *SimulationEngine {
	os.RemoveAll(name)
	sim := &SimulationEngine{
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
