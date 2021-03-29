package mock

import (
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common/consensus"
	"github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type BlockValidation interface {
	types.BlockInterface
	AddValidationField(validationData string) error
}

type ConsensusInterface interface {
	GetOneValidator() *consensus.Validator
	GetOneValidatorForEachConsensusProcess() map[int]*consensus.Validator
	ValidateProducerPosition(blk types.BlockInterface, lastProposerIdx int, committee []incognitokey.CommitteePublicKey, minCommitteeSize int) error
	ValidateProducerSig(block types.BlockInterface, consensusType string) error
	ValidateBlockCommitteSig(block types.BlockInterface, committee []incognitokey.CommitteePublicKey) error
	IsCommitteeInShard(byte) bool
}

type Consensus struct {
	consensusEngine *consensus_v2.Engine
}

func (c *Consensus) GetOneValidator() *consensus.Validator {
	return nil
}

func (c *Consensus) GetOneValidatorForEachConsensusProcess() map[int]*consensus.Validator {
	return nil
}

func (c *Consensus) ValidateProducerPosition(blk types.BlockInterface, lastProposerIdx int, committee []incognitokey.CommitteePublicKey, minCommitteeSize int) error {
	return c.consensusEngine.ValidateProducerPosition(blk, lastProposerIdx, committee, minCommitteeSize)
}

func (c *Consensus) ValidateProducerSig(block types.BlockInterface, consensusType string) error {
	return c.consensusEngine.ValidateProducerSig(block, consensusType)
}

func (c *Consensus) ValidateBlockCommitteSig(block types.BlockInterface, committee []incognitokey.CommitteePublicKey) error {
	return c.consensusEngine.ValidateBlockCommitteSig(block, committee)
}

func (c *Consensus) IsCommitteeInShard(sid byte) bool {
	return true
}
