package devframework

import (
	"fmt"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/multiview"
	"github.com/syndtr/goleveldb/leveldb"
)

func (s *NodeEngine) OnReceive(msgType int, f func(msg interface{})) {
	s.Network.listennerRegister[msgType] = append(s.Network.listennerRegister[msgType], f)
}

//note: this function is async, meaning that f function does not lock insert process
func (s *NodeEngine) OnInserted(blkType int, f func(msg interface{})) {
	s.listennerRegister[blkType] = append(s.listennerRegister[blkType], f)
}

func (s *NodeEngine) OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64, chainID int)) {
	fullmode := func() {
		var chainBestView multiview.View
		var chainFinalView multiview.View
		if chainID == -1 {
			chainBestView = s.bc.BeaconChain.GetBestView()
			chainFinalView = s.bc.BeaconChain.GetFinalView()
		} else {
			chainBestView = s.bc.ShardChain[chainID].GetBestView()
			chainFinalView = s.bc.ShardChain[chainID].GetFinalView()
		}
		waitingBlkHeight := uint64(blkHeight)
		if blkHeight == -1 {
			if isFinalized {
				waitingBlkHeight = chainFinalView.GetHeight()
			} else {
				waitingBlkHeight = chainBestView.GetHeight()
			}
		}
		go func() {
			for {
				if chainID == -1 {
					chainBestView = s.bc.BeaconChain.GetBestView()
					chainFinalView = s.bc.BeaconChain.GetFinalView()
				} else {
					chainBestView = s.bc.ShardChain[chainID].GetBestView()
					chainFinalView = s.bc.ShardChain[chainID].GetFinalView()
				}
				if (isFinalized && chainFinalView.GetHeight() >= waitingBlkHeight) || (!isFinalized && chainBestView.GetHeight() >= waitingBlkHeight) {
					if chainID == -1 {
						hash, err := s.bc.GetBeaconBlockHashByHeight(chainFinalView, chainBestView, waitingBlkHeight)
						if err == nil {
							f(s.bc, *hash, waitingBlkHeight, chainID)
							waitingBlkHeight++
						}
					} else {
						hash, err := s.bc.GetShardBlockHashByHeight(chainFinalView, chainBestView, waitingBlkHeight)
						if err == nil {
							f(s.bc, *hash, waitingBlkHeight, chainID)
							waitingBlkHeight++
						}
					}

				} else {
					time.Sleep(500 * time.Millisecond)
				}
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
				if chain.LocalHeight >= waitingBlkHeight {
					prefix := fmt.Sprintf("s-%v-%v", chainID, waitingBlkHeight)
					blkHash, err := s.userDB.Get([]byte(prefix), nil)
					if err != nil {
						if err == leveldb.ErrNotFound {
							time.Sleep(5 * time.Second)
							continue
						}
						panic(err)
					}
					hash, err := common.Hash{}.NewHash(blkHash)
					if err == nil {
						f(s.bc, *hash, waitingBlkHeight, chainID)
						waitingBlkHeight++
					}

				} else {
					time.Sleep(500 * time.Millisecond)
				}

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
