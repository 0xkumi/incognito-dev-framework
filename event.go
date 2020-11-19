package devframework

import (
	"time"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
)

func (s *SimulationEngine) OnReceive(msgType int, f func(msg interface{})) {
	s.Network.listennerRegister[msgType] = append(s.Network.listennerRegister[msgType], f)
}

//note: this function is async, meaning that f function does not lock insert process
func (s *SimulationEngine) OnInserted(blkType int, f func(msg interface{})) {
	s.listennerRegister[blkType] = append(s.listennerRegister[blkType], f)
}

func (s *SimulationEngine) OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64)) {
	chain := s.bc.GetChain(chainID)
	waitingBlkHeight := uint64(blkHeight)
	if blkHeight == -1 {
		if isFinalized {
			waitingBlkHeight = chain.GetFinalView().GetHeight()
		} else {
			waitingBlkHeight = chain.GetBestView().GetHeight()
		}
	}

	go func() {
		for {
			if (isFinalized && chain.GetFinalView().GetHeight() > waitingBlkHeight) || (!isFinalized && chain.GetBestView().GetHeight() > waitingBlkHeight) {
				if chainID == -1 {
					hash, err := s.bc.GetBeaconBlockHashByHeight(chain.GetFinalView(), chain.GetBestView(), waitingBlkHeight)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight)
					}
				} else {
					hash, err := s.bc.GetShardBlockHashByHeight(chain.GetFinalView(), chain.GetBestView(), waitingBlkHeight)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight)
					}
				}
			} else {
				time.Sleep(500 * time.Millisecond)
			}
			waitingBlkHeight++
		}
	}()
}
