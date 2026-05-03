package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type ChainValidator struct {
	client          *ethclient.Client
	contractAddress common.Address
	contractABI     abi.ABI
	config          *IndexerConfig
	
	blockCache      map[uint64]*CachedBlock
	cacheMutex      sync.RWMutex
	
	validationQueue chan uint64
	ctx             context.Context
}

type CachedBlock struct {
	BlockNumber  uint64
	BlockHash    string
	ParentHash   string
	Timestamp    time.Time
	Events       []ProcessedEvent
	IsValidated  bool
	IsOrphaned   bool
}

type ProcessedEvent struct {
	LogEntry        types.Log
	BlockHash       string
	ProductHash     string
	CurrentHash     string
	PreviousHash    string
	Status          models.ProductStatus
	Operator        string
	Timestamp       int64
	BlockNumber     uint64
	TransactionHash string
	EventIndex      uint
	IsProcessed     bool
	IsRolledBack    bool
}

type IndexerConfig struct {
	EthRPCURL          string
	ContractAddress    string
	BlockConfirmations uint64
	ReorgCheckInterval int
	MaxCacheBlocks     int
	StartBlock         uint64
}

func NewChainValidator(cfg *IndexerConfig) (*ChainValidator, error) {
	client, err := ethclient.Dial(cfg.EthRPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("contract address not configured")
	}

	contractAddress := common.HexToAddress(cfg.ContractAddress)

	contractABI, err := getContractABI()
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	if cfg.MaxCacheBlocks == 0 {
		cfg.MaxCacheBlocks = 1000
	}

	return &ChainValidator{
		client:          client,
		contractAddress: contractAddress,
		contractABI:     contractABI,
		config:          cfg,
		blockCache:      make(map[uint64]*CachedBlock),
		validationQueue: make(chan uint64, 100),
	}, nil
}

func (v *ChainValidator) Start(ctx context.Context) {
	v.ctx = ctx

	go v.processValidationQueue(ctx)

	go v.startPeriodicValidation(ctx)

	log.Println("Chain validator started")
}

func (v *ChainValidator) processValidationQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case blockNum := <-v.validationQueue:
			v.validateBlock(blockNum)
		}
	}
}

func (v *ChainValidator) startPeriodicValidation(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(v.config.ReorgCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.performFullChainValidation()
		}
	}
}

func (v *ChainValidator) validateBlock(blockNum uint64) error {
	v.cacheMutex.RLock()
	cached, exists := v.blockCache[blockNum]
	v.cacheMutex.RUnlock()

	if exists && cached.IsValidated {
		return nil
	}

	chainBlock, err := v.client.BlockByNumber(v.ctx, big.NewInt(int64(blockNum)))
	if err != nil {
		return fmt.Errorf("failed to get block %d: %v", blockNum, err)
	}

	storedBlock, err := v.getStoredBlock(blockNum)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to get stored block %d: %v", blockNum, err)
	}

	if storedBlock == nil {
		return v.cacheBlockFromChain(chainBlock)
	}

	chainHash := chainBlock.Hash().Hex()
	if chainHash != storedBlock.BlockHash {
		log.Printf("VALIDATION FAILURE: Block %d hash mismatch", blockNum)
		log.Printf("  Chain hash:  %s", chainHash)
		log.Printf("  Stored hash: %s", storedBlock.BlockHash)
		return fmt.Errorf("block hash mismatch at block %d", blockNum)
	}

	if !storedBlock.IsCanonical {
		log.Printf("WARNING: Block %d marked as non-canonical but hash matches", blockNum)
	}

	v.cacheMutex.Lock()
	if cached != nil {
		cached.IsValidated = true
	}
	v.cacheMutex.Unlock()

	return nil
}

func (v *ChainValidator) performFullChainValidation() (*models.ReorgAnalysis, error) {
	currentBlock, err := v.client.BlockNumber(v.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %v", err)
	}

	if currentBlock <= v.config.BlockConfirmations {
		return nil, nil
	}

	validationStart := currentBlock - v.config.BlockConfirmations

	lastConfirmed, err := v.getLastConfirmedBlock()
	if err != nil {
		lastConfirmed = v.config.StartBlock
	}

	if validationStart <= lastConfirmed {
		return nil, nil
	}

	analysis, err := v.analyzeChainFork(lastConfirmed, validationStart)
	if err != nil {
		return nil, fmt.Errorf("chain analysis failed: %v", err)
	}

	if analysis != nil && len(analysis.OldChainBlocks) > 0 {
		log.Printf("FORK DETECTED: type=%s, depth=%d, old_chain_blocks=%d, new_chain_blocks=%d",
			analysis.ForkType, analysis.ForkDepth, len(analysis.OldChainBlocks), len(analysis.NewChainBlocks))
		return analysis, nil
	}

	v.updateLastConfirmedBlock(validationStart)

	return nil, nil
}

func (v *ChainValidator) analyzeChainFork(startBlock, endBlock uint64) (*models.ReorgAnalysis, error) {
	var oldChainLinks []models.ChainLink
	var newChainLinks []models.ChainLink
	var lastCommonBlock uint64
	var lastCommonHash string
	var forkDetected bool

	for blockNum := endBlock; blockNum >= startBlock; blockNum-- {
		storedBlock, err := v.getStoredBlock(blockNum)
		if err == gorm.ErrRecordNotFound {
			if forkDetected {
				chainBlock, err := v.client.BlockByNumber(v.ctx, big.NewInt(int64(blockNum)))
				if err == nil {
					newChainLinks = append(newChainLinks, models.ChainLink{
						BlockNumber: blockNum,
						BlockHash:   chainBlock.Hash().Hex(),
						ParentHash:  chainBlock.ParentHash().Hex(),
						IsValid:     true,
					})
				}
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		chainBlock, err := v.client.BlockByNumber(v.ctx, big.NewInt(int64(blockNum)))
		if err != nil {
			log.Printf("Warning: failed to get chain block %d: %v", blockNum, err)
			continue
		}

		chainHash := chainBlock.Hash().Hex()

		if chainHash == storedBlock.BlockHash {
			if forkDetected {
				lastCommonBlock = blockNum
				lastCommonHash = chainHash
				break
			}
			continue
		}

		forkDetected = true

		oldChainLinks = append(oldChainLinks, models.ChainLink{
			BlockNumber: blockNum,
			BlockHash:   storedBlock.BlockHash,
			ParentHash:  storedBlock.ParentHash,
			IsValid:     false,
		})

		newChainLinks = append(newChainLinks, models.ChainLink{
			BlockNumber: blockNum,
			BlockHash:   chainHash,
			ParentHash:  chainBlock.ParentHash().Hex(),
			IsValid:     true,
		})
	}

	if !forkDetected || len(oldChainLinks) == 0 {
		return nil, nil
	}

	if lastCommonBlock == 0 && startBlock > 0 {
		lastCommonBlock = startBlock - 1
		commonBlock, err := v.getStoredBlock(lastCommonBlock)
		if err == nil {
			lastCommonHash = commonBlock.BlockHash
		}
	}

	forkType := v.classifyFork(oldChainLinks, newChainLinks)

	return &models.ReorgAnalysis{
		LastCommonBlock: lastCommonBlock,
		LastCommonHash:  lastCommonHash,
		OldChainBlocks:  oldChainLinks,
		NewChainBlocks:  newChainLinks,
		ForkType:        forkType,
		ForkDepth:       len(oldChainLinks),
	}, nil
}

func (v *ChainValidator) classifyFork(oldBlocks, newBlocks []models.ChainLink) models.ChainForkType {
	if len(oldBlocks) == 1 && len(newBlocks) == 1 {
		return models.ForkTypeOrphan
	}

	if len(oldBlocks) <= 3 {
		return models.ForkTypeTemporary
	}

	return models.ForkTypeReorg
}

func (v *ChainValidator) cacheBlockFromChain(block *types.Block) error {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()

	cached := &CachedBlock{
		BlockNumber: block.NumberU64(),
		BlockHash:   block.Hash().Hex(),
		ParentHash:  block.ParentHash().Hex(),
		Timestamp:   time.Unix(int64(block.Time()), 0),
		IsValidated: true,
	}

	v.blockCache[block.NumberU64()] = cached

	v.pruneCache()

	return nil
}

func (v *ChainValidator) pruneCache() {
	if len(v.blockCache) <= v.config.MaxCacheBlocks {
		return
	}

	var oldestBlock uint64 = ^uint64(0)
	for num := range v.blockCache {
		if num < oldestBlock {
			oldestBlock = num
		}
	}

	delete(v.blockCache, oldestBlock)
}

func (v *ChainValidator) getStoredBlock(blockNum uint64) (*models.ChainBlock, error) {
	var block models.ChainBlock
	err := database.DB.Where("block_number = ? AND is_canonical = ?", blockNum, true).First(&block).Error
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (v *ChainValidator) getLastConfirmedBlock() (uint64, error) {
	var state models.SyncState
	err := database.DB.First(&state).Error
	if err != nil {
		return 0, err
	}
	return state.LastConfirmedBlock, nil
}

func (v *ChainValidator) updateLastConfirmedBlock(blockNum uint64) {
	database.DB.Model(&models.SyncState{}).
		Where("1 = 1").
		Update("last_confirmed_block", blockNum)
}

func (v *ChainValidator) GetBlockEvents(blockNum uint64) ([]ProcessedEvent, error) {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(blockNum)),
		ToBlock:   big.NewInt(int64(blockNum)),
		Addresses: []common.Address{v.contractAddress},
	}

	logs, err := v.client.FilterLogs(v.ctx, query)
	if err != nil {
		return nil, err
	}

	var events []ProcessedEvent
	for _, logEntry := range logs {
		if logEntry.Removed {
			continue
		}

		event, err := v.parseEvent(logEntry)
		if err != nil {
			log.Printf("Failed to parse event in block %d: %v", blockNum, err)
			continue
		}

		events = append(events, *event)
	}

	return events, nil
}

func (v *ChainValidator) parseEvent(logEntry types.Log) (*ProcessedEvent, error) {
	event := make(map[string]interface{})

	err := v.contractABI.UnpackIntoMap(event, "ProductStateChanged", logEntry.Data)
	if err != nil {
		return nil, err
	}

	status, ok := event["status"].(uint8)
	if !ok {
		return nil, fmt.Errorf("failed to parse status")
	}

	operator, ok := event["operator"].(common.Address)
	if !ok {
		return nil, fmt.Errorf("failed to parse operator")
	}

	timestamp, ok := event["timestamp"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to parse timestamp")
	}

	previousHash, ok := event["previousHash"].([32]byte)
	if !ok {
		return nil, fmt.Errorf("failed to parse previousHash")
	}

	currentHash, ok := event["currentHash"].([32]byte)
	if !ok {
		return nil, fmt.Errorf("failed to parse currentHash")
	}

	var productHash [32]byte
	if len(logEntry.Topics) > 1 {
		copy(productHash[:], logEntry.Topics[1][:])
	}

	return &ProcessedEvent{
		LogEntry:        logEntry,
		BlockHash:       logEntry.BlockHash.Hex(),
		ProductHash:     "0x" + hex.EncodeToString(productHash[:]),
		CurrentHash:     "0x" + hex.EncodeToString(currentHash[:]),
		PreviousHash:    "0x" + hex.EncodeToString(previousHash[:]),
		Status:          models.ProductStatus(status),
		Operator:        operator.Hex(),
		Timestamp:       timestamp.Int64(),
		BlockNumber:     logEntry.BlockNumber,
		TransactionHash: logEntry.TxHash.Hex(),
		EventIndex:      logEntry.Index,
	}, nil
}

func (v *ChainValidator) GetClient() *ethclient.Client {
	return v.client
}

func (v *ChainValidator) GetContractAddress() common.Address {
	return v.contractAddress
}

func (v *ChainValidator) GetContractABI() abi.ABI {
	return v.contractABI
}

func getContractABI() (abi.ABI, error) {
	abiJSON := `[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "bytes32",
					"name": "productHash",
					"type": "bytes32"
				},
				{
					"indexed": true,
					"internalType": "bytes32",
					"name": "serialNumberHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "enum SupplyChainTraceability.ProductStatus",
					"name": "status",
					"type": "uint8"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "operator",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "timestamp",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "bytes32",
					"name": "previousHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "bytes32",
					"name": "currentHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "blockNumber",
					"type": "uint256"
				}
			],
			"name": "ProductStateChanged",
			"type": "event"
		}
	]`
	
	return abi.JSON(strings.NewReader(abiJSON))
}
