package devframework

import (
	"os"

	"github.com/incognitochain/incognito-chain/blockchain/committeestate"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/peerv2"
	"github.com/incognitochain/incognito-chain/portal"
	"github.com/incognitochain/incognito-chain/portal/portalrelaying"
	portalcommonv3 "github.com/incognitochain/incognito-chain/portal/portalv3/common"
	portalprocessv3 "github.com/incognitochain/incognito-chain/portal/portalv3/portalprocess"
	portaltokensv3 "github.com/incognitochain/incognito-chain/portal/portalv3/portaltokens"
	portalprocessv4 "github.com/incognitochain/incognito-chain/portal/portalv4/portalprocess"
	portaltokensv4 "github.com/incognitochain/incognito-chain/portal/portalv4/portaltokens"
	"github.com/incognitochain/incognito-chain/syncker"

	"github.com/incognitochain/incognito-chain/syncker/finishsync"
	"github.com/incognitochain/incognito-chain/txpool"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/rpcserver"
	"github.com/incognitochain/incognito-chain/rpcserver/rpcservice"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/jrick/logrotate/rotator"
)

var (
	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator *rotator.Rotator

	backendLog             = common.NewBackend(logWriter{})
	dbLogger               = backendLog.Logger("Database log", false)
	blockchainLogger       = backendLog.Logger("BlockChain log", false)
	bridgeLogger           = backendLog.Logger("DeBridge log", false)
	rpcLogger              = backendLog.Logger("RPC log", false)
	rpcServiceLogger       = backendLog.Logger("RPC service log", false)
	rpcServiceBridgeLogger = backendLog.Logger("RPC service DeBridge log", false)
	transactionLogger      = backendLog.Logger("Transaction log", false)
	// privacyLogger          = backendLog.Logger("Privacy log", false)
	mempoolLogger        = backendLog.Logger("Mempool log", false)
	synckerLogger        = backendLog.Logger("Syncker log", false)
	highwayLogger        = backendLog.Logger("Highway", false)
	consensusLogger      = backendLog.Logger("Consensus log", false)
	privacyV1Logger      = backendLog.Logger("Privacy V1 log ", false)
	privacyV2Logger      = backendLog.Logger("Privacy V2 log ", false)
	committeeStateLogger = backendLog.Logger("Committee State log ", false)
	pdexLogger           = backendLog.Logger("Pdex log ", false)
	finishSyncLogger     = backendLog.Logger("Finish Sync log ", false)

	portalLogger          = backendLog.Logger("Portal log ", false)
	portalRelayingLogger  = backendLog.Logger("Portal relaying log ", false)
	portalV3CommonLogger  = backendLog.Logger("Portal v3 common log ", false)
	portalV3ProcessLogger = backendLog.Logger("Portal v3 process log ", false)
	portalV3TokenLogger   = backendLog.Logger("Portal v3 token log ", false)
	txPoolLogger          = backendLog.Logger("Txpool log ", false)

	portalV4ProcessLogger = backendLog.Logger("Portal v4 process log ", false)
	portalV4TokenLogger   = backendLog.Logger("Portal v4 token log ", false)

	disableStdoutLog = false
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	if !disableStdoutLog {
		os.Stdout.Write(p)
	}
	if logRotator != nil {
		logRotator.Write(p)
	}

	return len(p), nil
}

func init() {
	incdb.Logger.Init(dbLogger)
	blockchain.Logger.Init(blockchainLogger)
	blockchain.BLogger.Init(bridgeLogger)
	rpcserver.Logger.Init(rpcLogger)
	rpcservice.Logger.Init(rpcServiceLogger)
	rpcservice.BLogger.Init(rpcServiceBridgeLogger)
	transaction.Logger.Init(transactionLogger)
	committeestate.Logger.Init(committeeStateLogger)
	// privacy.Logger.Init(privacyLogger)
	mempool.Logger.Init(mempoolLogger)
	syncker.Logger.Init(synckerLogger)
	finishsync.Logger.Init(finishSyncLogger)
	peerv2.Logger.Init(highwayLogger)
	consensus.Logger.Init(consensusLogger)
	privacy.LoggerV1.Init(privacyV1Logger)
	privacy.LoggerV2.Init(privacyV2Logger)
	pdex.Logger.Init(pdexLogger)

	portal.Logger.Init(portalLogger)
	portalrelaying.Logger.Init(portalRelayingLogger)
	portalcommonv3.Logger.Init(portalV3CommonLogger)
	portalprocessv3.Logger.Init(portalV3ProcessLogger)
	portaltokensv3.Logger.Init(portalV3TokenLogger)

	portalprocessv4.Logger.Init(portalV4ProcessLogger)
	portaltokensv4.Logger.Init(portalV4TokenLogger)

	txpool.Logger.Init(txPoolLogger)
}

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemLoggers = map[string]common.Logger{
	"DABA":              dbLogger,
	"BLOC":              blockchainLogger,
	"DEBR":              bridgeLogger,
	"RPCS":              rpcLogger,
	"RPCSservice":       rpcServiceLogger,
	"RPCSbridgeservice": rpcServiceBridgeLogger,
	"TRAN":              transactionLogger,
	// "PRIV":              privacyLogger,
	"MEMP":            mempoolLogger,
	"CONS":            consensusLogger,
	"COMS":            committeeStateLogger,
	"PORTAL":          portalLogger,
	"PORTALRELAYING":  portalRelayingLogger,
	"PORTALV3COMMON":  portalV3CommonLogger,
	"PORTALV3PROCESS": portalV3ProcessLogger,
	"PORTALV3TOKENS":  portalV3TokenLogger,
	"PORTALV4PROCESS": portalV4ProcessLogger,
	"PORTALV4TOKENS":  portalV4TokenLogger,
	"FINS":            finishSyncLogger,
	"HW":              highwayLogger,
	"SYK":             synckerLogger,
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logFile string) {
	// logDir, _ := filepath.Split(logFile)
	// err := os.MkdirAll(logDir, 0700)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
	// 	os.Exit(common.ExitByLogging)
	// }
	// r, err := rotator.New(logFile, 10*1024, false, 3)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
	// 	os.Exit(common.ExitByLogging)
	// }

	// logRotator = r
	logRotator = &rotator.Rotator{}
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := common.LevelFromString(logLevel)
	logger.SetLevel(level)
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

type MainLogger struct {
	log common.Logger
}

func (mainLogger *MainLogger) Init(inst common.Logger) {
	mainLogger.log = inst
}

// Global instant to use
var Logger = MainLogger{}
