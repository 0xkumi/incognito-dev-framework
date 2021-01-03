package devframework

import (
	"fmt"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
)

func (s *NodeEngine) OnReceive(msgType int, f func(msg interface{})) {
	s.Network.listennerRegister[msgType] = append(s.Network.listennerRegister[msgType], f)
}

//note: this function is async, meaning that f function does not lock insert process
func (s *NodeEngine) OnInserted(blkType int, f func(msg interface{})) {
	s.listennerRegister[blkType] = append(s.listennerRegister[blkType], f)
}

func (s *NodeEngine) OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64)) {
	fullmode := func() {
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
		return
	}
	lightmode := func() {
		chain := s.lightNodeData.Shards[byte(chainID)]
		waitingBlkHeight := uint64(blkHeight)
		if blkHeight == -1 {
			waitingBlkHeight = chain.LocalHeight
		}
		if blkHeight == 0 {
			waitingBlkHeight = 1
		}
		go func() {
			for {
				chain := s.lightNodeData.Shards[byte(chainID)]
				if chain.LocalHeight > waitingBlkHeight {
					prefix := fmt.Sprintf("s-%v-%v", chainID, waitingBlkHeight)
					blkHash, err := s.userDB.Get([]byte(prefix), nil)
					if err != nil {
						panic(err)
					}
					hash, err := common.Hash{}.NewHash(blkHash)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight)
					}
				} else {
					time.Sleep(500 * time.Millisecond)
				}
				waitingBlkHeight++
			}
		}()
		return
	}

	if chainID == -1 {
		fullmode()
		return
	} else {
		if s.appNodeMode == "full" {
			fullmode()
			return
		}
		if s.appNodeMode == "light" {
			lightmode()
			return
		}
	}
}
