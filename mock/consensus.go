package mock

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/consensus"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type BlockValidation interface {
	common.BlockInterface
	AddValidationField(validationData string) error
}

type ConsensusInterface interface {
	GetOneValidator() *consensus.Validator
	GetOneValidatorForEachConsensusProcess() map[int]*consensus.Validator
	ValidateProducerPosition(blk common.BlockInterface, lastProposerIdx int, committee []incognitokey.CommitteePublicKey, minCommitteeSize int) error
	ValidateProducerSig(block common.BlockInterface, consensusType string) error
	ValidateBlockCommitteSig(block common.BlockInterface, committee []incognitokey.CommitteePublicKey) error
	IsCommitteeInShard(byte) bool
}

type Consensus struct {
	Blockchain *blockchain.BlockChain
}

func (c *Consensus) GetOneValidator() *consensus.Validator {
	return nil
}

func (c *Consensus) GetOneValidatorForEachConsensusProcess() map[int]*consensus.Validator {
	return nil
}

func (c *Consensus) ValidateProducerPosition(blk common.BlockInterface, lastProposerIdx int, committee []incognitokey.CommitteePublicKey, minCommitteeSize int) error {
	return nil
}

func (c *Consensus) ValidateProducerSig(block common.BlockInterface, consensusType string) error {
	return nil
}

func (c *Consensus) ValidateBlockCommitteSig(block common.BlockInterface, committee []incognitokey.CommitteePublicKey) error {
	return nil
}

func (c *Consensus) IsCommitteeInShard(byte) bool {
	return true
}
