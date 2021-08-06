package mock

import (
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common/consensus"
	"github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/incognitokey"
	portalprocessv4 "github.com/incognitochain/incognito-chain/portal/portalv4/portalprocess"
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
	ExtractBridgeValidationData(block types.BlockInterface) ([][]byte, []int, error)
	GetAllMiningPublicKeys() []string
	GetAllValidatorKeyState() map[string]consensus.MiningState
	GetUserRole() (string, string, int)
	GetCurrentMiningPublicKey() (string, string)
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

func (c *Consensus) ExtractBridgeValidationData(block types.BlockInterface) ([][]byte, []int, error) {
	return nil, nil, nil
}
func (c *Consensus) GetAllMiningPublicKeys() []string {
	return nil
}

func (c *Consensus) GetAllValidatorKeyState() map[string]consensus.MiningState {
	return nil
}
func (c *Consensus) GetUserRole() (string, string, int) {
	return "", "", 0
}
func (c *Consensus) GetCurrentMiningPublicKey() (string, string) {
	return "", ""
}
func (c *Consensus) ExtractPortalV4ValidationData(block types.BlockInterface) ([]*portalprocessv4.PortalSig, error) {
	return nil, nil
}
