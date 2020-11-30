package devframework

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/consensus"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/syncker"
)

type Execute struct {
	sim          *SimulationEngine
	appliedChain []int
}

func (exec *Execute) GenerateBlock(args ...interface{}) *Execute {
	args = append(args, exec)
	exec.sim.GenerateBlock(args...)
	return exec
}

func (exec *Execute) NextRound() {
	exec.sim.NextRound()
}

func (sim *SimulationEngine) ApplyChain(chain_array ...int) *Execute {
	return &Execute{
		sim,
		chain_array,
	}
}

type Syncker interface {
	GetCrossShardBlocksForShardProducer(toShard byte, limit map[byte][]uint64) map[byte][]interface{}
	GetCrossShardBlocksForShardValidator(toShard byte, list map[byte][]uint64) (map[byte][]interface{}, error)
	SyncMissingBeaconBlock(ctx context.Context, peerID string, fromHash common.Hash)
	SyncMissingShardBlock(ctx context.Context, peerID string, sid byte, fromHash common.Hash)
	Init(*syncker.SynckerManagerConfig)
	InsertCrossShardBlock(block *blockchain.CrossShardBlock)
}

type Consensus interface {
	GetOneValidator() *consensus.Validator
	GetOneValidatorForEachConsensusProcess() map[int]*consensus.Validator
	ValidateProducerPosition(blk common.BlockInterface, lastProposerIdx int, committee []incognitokey.CommitteePublicKey, minCommitteeSize int) error
	ValidateProducerSig(block common.BlockInterface, consensusType string) error
	ValidateBlockCommitteSig(block common.BlockInterface, committee []incognitokey.CommitteePublicKey) error
	IsCommitteeInShard(byte) bool
}

const (
	MSG_TX = iota
	MSG_TX_PRIVACYTOKEN

	MSG_BLOCK_SHARD
	MSG_BLOCK_BEACON
	MSG_BLOCK_XSHARD

	MSG_PEER_STATE

	MSG_BFT
)

const (
	BLK_BEACON = iota
	BLK_SHARD
)
