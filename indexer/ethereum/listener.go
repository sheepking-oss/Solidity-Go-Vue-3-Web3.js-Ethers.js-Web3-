package ethereum

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"supply-chain-indexer/config"
	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type Indexer struct {
	client             *ethclient.Client
	contractAddress    common.Address
	contractABI        abi.ABI
	config             *config.Config
	
	lastSyncedBlock    uint64
	lastConfirmedBlock  uint64
	
	blockCache          map[uint64]*blockInfo
	pendingEvents       map[uint64][]*pendingEventData
	cacheMutex          sync.RWMutex
	
	ctx                 context.Context
	cancel              context.CancelFunc
}

type blockInfo struct {
	blockNumber uint64
	blockHash   string
	parentHash  string
	timestamp   time.Time
}

type pendingEventData struct {
	logEntry     types.Log
	blockHash  string
}

func NewIndexer(cfg *config.Config) (*Indexer, error) {
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

	var lastSyncedBlock, lastConfirmedBlock uint64 = cfg.StartBlock, cfg.StartBlock
	syncState, err := database.GetSyncState()
	if err == nil {
		lastSyncedBlock = syncState.LastSyncedBlock
		lastConfirmedBlock = syncState.LastConfirmedBlock
		log.Printf("Loaded sync state: last_synced=%d, last_confirmed=%d", lastSyncedBlock, lastConfirmedBlock)
	} else if err != gorm.ErrRecordNotFound {
		log.Printf("Warning: failed to load sync state: %v", err)
	}

	return &Indexer{
		client:            client,
		contractAddress:   contractAddress,
		contractABI:       contractABI,
		config:            cfg,
		lastSyncedBlock:   lastSyncedBlock,
		lastConfirmedBlock: lastConfirmedBlock,
		blockCache:         make(map[uint64]*blockInfo),
		pendingEvents:      make(map[uint64][]*pendingEventData),
	}, nil
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

func (i *Indexer) Start(ctx context.Context) error {
	i.ctx, i.cancel = context.WithCancel(ctx)
	defer i.cancel()

	log.Println("Starting enhanced event indexer with reorg protection...")
	log.Printf("Block confirmations: %d", i.config.BlockConfirmations)

	eventSignature := i.contractABI.Events["ProductStateChanged"].ID

	query := ethereum.FilterQuery{
		Addresses: []common.Address{i.contractAddress},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	logsChan := make(chan types.Log)

	sub, err := i.client.SubscribeFilterLogs(i.ctx, query, logsChan)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	go i.startReorgChecker(i.ctx)

	go i.startConfirmationProcessor(i.ctx)

	go i.syncPastEvents(i.ctx)

	log.Println("Indexer started, listening for events...")

	for {
		select {
		case <-i.ctx.Done():
			log.Println("Indexer shutting down...")
			return nil
		case err := <-sub.Err():
			log.Printf("Subscription error: %v\n", err)
			return err
		case vLog := <-logsChan:
			i.processNewLog(vLog)
		}
	}
}

func (i *Indexer) startReorgChecker(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(i.config.ReorgCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Reorg checker shutting down...")
			return
		case <-ticker.C:
			if err := i.checkForReorg(ctx); err != nil {
				log.Printf("Reorg check error: %v", err)
			}
		}
	}
}

func (i *Indexer) checkForReorg(ctx context.Context) error {
	currentBlock, err := i.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %v", err)
	}

	if currentBlock <= i.config.BlockConfirmations {
		return nil
	}

	checkBlock := currentBlock - i.config.BlockConfirmations
	if checkBlock <= i.lastConfirmedBlock {
		return nil
	}

	var storedBlocks, err := i.getStoredBlocksFrom(i.lastConfirmedBlock)
	if err != nil {
		return fmt.Errorf("failed to get stored blocks: %v", err)
	}

	if len(storedBlocks) == 0 {
		return nil
	}

	var reorgStartBlock uint64
	var reorgDetected := false

	for _, storedBlock := range storedBlocks {
		chainBlock, err := i.client.BlockByNumber(ctx, big.NewInt(int64(storedBlock.BlockNumber)))
		if err != nil {
			log.Printf("Warning: failed to get block %d: %v", storedBlock.BlockNumber, err)
			continue
		}

		chainHash := chainBlock.Hash().Hex()
		if chainHash != storedBlock.BlockHash {
			log.Printf("REORG DETECTED: Block %d hash mismatch", storedBlock.BlockNumber)
			log.Printf("  Stored: %s", storedBlock.BlockHash)
			log.Printf("  Chain:  %s", chainHash)
			reorgStartBlock = storedBlock.BlockNumber
			reorgDetected = true
			break
		}
	}

	if reorgDetected {
		if err := i.handleReorg(ctx, reorgStartBlock); err != nil {
			return fmt.Errorf("failed to handle reorg: %v", err)
		}
	} else {
		newConfirmedBlock := storedBlocks[len(storedBlocks)-1].BlockNumber
		if newConfirmedBlock > i.lastConfirmedBlock {
			i.lastConfirmedBlock = newConfirmedBlock
			i.confirmBlocksUpTo(newConfirmedBlock)
			database.UpdateSyncState(i.lastSyncedBlock, i.lastConfirmedBlock)
		}
	}

	return nil
}

func (i *Indexer) handleReorg(ctx context.Context, forkBlock uint64) error {
	log.Printf("=== HANDLING REORG AT BLOCK %d ===", forkBlock)

	var affectedEvents int64
	result := database.DB.Model(&models.ProductState{}).
		Where("block_number >= ?", forkBlock).
		Count(&affectedEvents)
	
	if result.Error != nil {
		return fmt.Errorf("failed to count affected events: %v", result.Error)
	}

	log.Printf("Found %d events that need to be rolled back", affectedEvents)

	tx := database.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	if err := tx.Where("block_number >= ?", forkBlock).
		Delete(&models.ProductState{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete orphaned events: %v", err)
	}

	if err := tx.Model(&models.BlockRecord{}).
		Where("block_number >= ?", forkBlock).
		Updates(map[string]interface{}{
			"status":       models.BlockStatusOrphaned,
			"is_canonical": false,
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update block records: %v", err)
	}

	reorgEvent := &models.ReorgEvent{
		FromBlock:      forkBlock,
		ToBlock:        i.lastSyncedBlock,
		AffectedEvents: affectedEvents,
		Reason:         "Chain reorg detected - hash mismatch",
		CreatedAt:      time.Now(),
	}

	if err := tx.Create(reorgEvent).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record reorg event: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	i.cacheMutex.Lock()
	for blockNum := range i.lastSyncedBlock; blockNum >= forkBlock; blockNum-- {
		delete(i.blockCache, blockNum)
		delete(i.pendingEvents, blockNum)
	}
	i.cacheMutex.Unlock()

	newSyncedBlock := forkBlock - 1
	if newSyncedBlock < i.config.StartBlock {
		newSyncedBlock = i.config.StartBlock
	}
	i.lastSyncedBlock = newSyncedBlock
	i.lastConfirmedBlock = newSyncedBlock

	database.UpdateSyncState(i.lastSyncedBlock, i.lastConfirmedBlock)

	log.Printf("=== REORG HANDLING COMPLETE ===")
	log.Printf("Rolled back to block %d", newSyncedBlock)

	go i.syncPastEvents(ctx)

	return nil
}

func (i *Indexer) getStoredBlocksFrom(startBlock uint64) ([]models.BlockRecord, error) {
	var blocks []models.BlockRecord
	err := database.DB.
		Where("block_number >= ? AND is_canonical = ?", startBlock, true).
		Order("block_number ASC").
		Find(&blocks).Error
	return blocks, err
}

func (i *Indexer) confirmBlocksUpTo(confirmedBlock uint64) {
	log.Printf("Confirming blocks up to: %d", confirmedBlock)

	i.cacheMutex.RLock()
	defer i.cacheMutex.RUnlock()

	for blockNum, events := range i.pendingEvents {
		if blockNum <= confirmedBlock {
			for _, eventData := range events {
				i.confirmAndSaveEvent(eventData.logEntry, eventData.blockHash)
			}

			blockInfo, exists := i.blockCache[blockNum]
			if exists {
				i.recordConfirmedBlock(blockInfo)
			}

			delete(i.pendingEvents, blockNum)
			delete(i.blockCache, blockNum)
		}
	}
}

func (i *Indexer) recordConfirmedBlock(info *blockInfo) {
	blockRecord := &models.BlockRecord{
		BlockNumber: info.blockNumber,
		BlockHash:   info.blockHash,
		ParentHash:  info.parentHash,
		Status:      models.BlockStatusConfirmed,
		IsCanonical: true,
		CreatedAt:   info.timestamp,
		UpdatedAt:   time.Now(),
	}

	if err := database.DB.Create(blockRecord).Error; err != nil {
		log.Printf("Failed to record confirmed block: %v", err)
	}
}

func (i *Indexer) startConfirmationProcessor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentBlock, err := i.client.BlockNumber(ctx)
			if err != nil {
				continue
			}

			if currentBlock > i.config.BlockConfirmations {
				confirmedThreshold := currentBlock - i.config.BlockConfirmations
				if confirmedThreshold > i.lastConfirmedBlock {
					i.confirmBlocksUpTo(confirmedThreshold)
					i.lastConfirmedBlock = confirmedThreshold
					database.UpdateSyncState(i.lastSyncedBlock, i.lastConfirmedBlock)
				}
			}
		}
	}
}

func (i *Indexer) processNewLog(vLog types.Log) {
	blockHash := vLog.BlockHash.Hex()

	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()

	if _, exists := i.blockCache[vLog.BlockNumber]; !exists {
		block, err := i.client.BlockByHash(i.ctx, vLog.BlockHash)
		if err != nil {
			log.Printf("Failed to get block info: %v", err)
			return
		}

		i.blockCache[vLog.BlockNumber] = &blockInfo{
			blockNumber: vLog.BlockNumber,
			blockHash:   blockHash,
			parentHash:  block.ParentHash().Hex(),
			timestamp:   time.Now(),
		}
	}

	i.pendingEvents[vLog.BlockNumber] = append(
		i.pendingEvents[vLog.BlockNumber],
		&pendingEventData{
			logEntry:    vLog,
			blockHash:   blockHash,
		},
	)

	if vLog.BlockNumber > i.lastSyncedBlock {
		i.lastSyncedBlock = vLog.BlockNumber
	}

	log.Printf("Event cached (pending confirmation): block=%d, tx=%s",
		vLog.BlockNumber, vLog.TxHash.Hex())
}

func (i *Indexer) confirmAndSaveEvent(vLog types.Log, blockHash string) {
	event := make(map[string]interface{})

	err := i.contractABI.UnpackIntoMap(event, "ProductStateChanged", vLog.Data)
	if err != nil {
		log.Printf("Failed to unpack log: %v\n", err)
		return
	}

	var productHash [32]byte
	var serialNumberHash [32]byte

	if len(vLog.Topics) > 1 {
		copy(productHash[:], vLog.Topics[1][:])
	}
	if len(vLog.Topics) > 2 {
		copy(serialNumberHash[:], vLog.Topics[2][:])
	}

	status, ok := event["status"].(uint8)
	if !ok {
		log.Printf("Failed to parse status\n")
		return
	}

	operator, ok := event["operator"].(common.Address)
	if !ok {
		log.Printf("Failed to parse operator\n")
		return
	}

	timestamp, ok := event["timestamp"].(*big.Int)
	if !ok {
		log.Printf("Failed to parse timestamp\n")
		return
	}

	previousHash, ok := event["previousHash"].([32]byte)
	if !ok {
		log.Printf("Failed to parse previousHash\n")
		return
	}

	currentHash, ok := event["currentHash"].([32]byte)
	if !ok {
		log.Printf("Failed to parse currentHash\n")
		return
	}

	productState := &models.ProductState{
		ProductHash:     "0x" + hex.EncodeToString(productHash[:]),
		SerialNumber:    "", 
		CurrentHash:     "0x" + hex.EncodeToString(currentHash[:]),
		PreviousHash:    "0x" + hex.EncodeToString(previousHash[:]),
		Status:          models.ProductStatus(status),
		StatusText:      models.ProductStatus(status).String(),
		Operator:        operator.Hex(),
		Timestamp:       timestamp.Int64(),
		BlockNumber:     vLog.BlockNumber,
		TransactionHash: vLog.TxHash.Hex(),
		EventIndex:      vLog.Index,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	var existing models.ProductState
	err = database.DB.Where("current_hash = ?", productState.CurrentHash).First(&existing).Error
	if err == nil {
		log.Printf("Event already processed: %s\n", productState.CurrentHash)
		return
	}

	if err != gorm.ErrRecordNotFound {
		log.Printf("Error checking existing record: %v\n", err)
		return
	}

	if err := database.DB.Create(productState).Error; err != nil {
		log.Printf("Failed to save product state: %v\n", err)
		return
	}

	log.Printf("Event confirmed and saved: status=%s, productHash=%s, block=%d\n",
		productState.StatusText, productState.ProductHash, vLog.BlockNumber)
}

func (i *Indexer) syncPastEvents(ctx context.Context) {
	log.Println("Syncing past events...")

	currentBlock, err := i.client.BlockNumber(ctx)
	if err != nil {
		log.Printf("Failed to get current block: %v\n", err)
		return
	}

	startSyncFrom := i.lastSyncedBlock
	if startSyncFrom >= currentBlock {
		log.Println("No past events to sync")
		return
	}

	eventSignature := i.contractABI.Events["ProductStateChanged"].ID

	batchSize := uint64(1000)

	for fromBlock := startSyncFrom + 1; fromBlock <= currentBlock; fromBlock += batchSize {
		toBlock := fromBlock + batchSize - 1
		if toBlock > currentBlock {
			toBlock = currentBlock
		}

		log.Printf("Syncing blocks %d to %d...", fromBlock, toBlock)

		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(fromBlock)),
			ToBlock:   big.NewInt(int64(toBlock)),
			Addresses: []common.Address{i.contractAddress},
			Topics:    [][]common.Hash{{eventSignature}},
		}

		logs, err := i.client.FilterLogs(ctx, query)
		if err != nil {
			log.Printf("Failed to filter logs for blocks %d-%d: %v\n", fromBlock, toBlock, err)
			continue
		}

		log.Printf("Found %d events in blocks %d-%d\n", len(logs), fromBlock, toBlock)

		for _, vLog := range logs {
			if vLog.Removed {
				continue
			}

			blockNum := vLog.BlockNumber
			if currentBlock-blockNum >= i.config.BlockConfirmations {
				i.confirmAndSaveEvent(vLog, vLog.BlockHash.Hex())
			} else {
				i.processNewLog(vLog)
			}
		}

		if toBlock > i.lastSyncedBlock {
			i.lastSyncedBlock = toBlock
		}
	}

	confirmedThreshold := currentBlock
	if confirmedThreshold > i.config.BlockConfirmations {
		confirmedThreshold = currentBlock - i.config.BlockConfirmations
	}
	if confirmedThreshold > i.lastConfirmedBlock {
		i.lastConfirmedBlock = confirmedThreshold
	}

	database.UpdateSyncState(i.lastSyncedBlock, i.lastConfirmedBlock)

	log.Println("Past events sync completed")
}

func Bytes32ToHex(b [32]byte) string {
	return "0x" + hex.EncodeToString(b[:])
}

func (i *Indexer) GetPendingEventCount() int {
	i.cacheMutex.RLock()
	defer i.cacheMutex.RUnlock()
	
	count := 0
	for _, events := range i.pendingEvents {
		count += len(events)
	}
	return count
}

func (i *Indexer) GetSyncStatus() map[string]interface{} {
	i.cacheMutex.RLock()
	defer i.cacheMutex.RUnlock()
	
	return map[string]interface{}{
		"last_synced_block":    i.lastSyncedBlock,
		"last_confirmed_block": i.lastConfirmedBlock,
		"pending_blocks":       len(i.blockCache),
		"pending_events":       i.GetPendingEventCount(),
		"block_confirmations":  i.config.BlockConfirmations,
	}
}

type PendingEventRecord struct {
	ID              uint   `json:"id"`
	BlockNumber     uint64 `json:"block_number"`
	TransactionHash string `json:"transaction_hash"`
	CreatedAt       string `json:"created_at"`
}

func (i *Indexer) GetPendingEvents() []PendingEventRecord {
	i.cacheMutex.RLock()
	defer i.cacheMutex.RUnlock()
	
	var records []PendingEventRecord
	for blockNum, events := range i.pendingEvents {
		for idx, evt := range events {
			records = append(records, PendingEventRecord{
				ID:              uint(idx),
				BlockNumber:     blockNum,
				TransactionHash: evt.logEntry.TxHash.Hex(),
				CreatedAt:       time.Now().Format(time.RFC3339),
			})
		}
	}
	return records
}

func (i *Indexer) SerializePendingEvent(event *pendingEventData) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"block_number": event.logEntry.BlockNumber,
		"block_hash":   event.blockHash,
		"tx_hash":      event.logEntry.TxHash.Hex(),
		"index":        event.logEntry.Index,
		"data":         event.logEntry.Data,
		"topics":       event.logEntry.Topics,
	})
}
