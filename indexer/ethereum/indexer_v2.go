package ethereum

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"supply-chain-indexer/config"
	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type EnhancedIndexer struct {
	validator       *ChainValidator
	rollbackMgr     *RollbackManager
	config          *config.Config

	lastSyncedBlock    uint64
	lastConfirmedBlock uint64

	pendingEvents   map[uint64][]*ProcessedEvent
	eventMutex      sync.RWMutex

	ctx             context.Context
	cancel          context.CancelFunc

	checkpointInterval uint64
	lastCheckpoint     uint64
}

func NewEnhancedIndexer(cfg *config.Config) (*EnhancedIndexer, error) {
	validatorCfg := &IndexerConfig{
		EthRPCURL:          cfg.EthRPCURL,
		ContractAddress:    cfg.ContractAddress,
		BlockConfirmations: cfg.BlockConfirmations,
		ReorgCheckInterval: cfg.ReorgCheckInterval,
		StartBlock:         cfg.StartBlock,
		MaxCacheBlocks:     1000,
	}

	validator, err := NewChainValidator(validatorCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain validator: %v", err)
	}

	rollbackMgr := NewRollbackManager(validator)

	var lastSynced, lastConfirmed uint64 = cfg.StartBlock, cfg.StartBlock
	syncState, err := database.GetSyncState()
	if err == nil {
		lastSynced = syncState.LastSyncedBlock
		lastConfirmed = syncState.LastConfirmedBlock
		log.Printf("Loaded sync state: last_synced=%d, last_confirmed=%d", lastSynced, lastConfirmed)
	}

	return &EnhancedIndexer{
		validator:           validator,
		rollbackMgr:         rollbackMgr,
		config:              cfg,
		lastSyncedBlock:     lastSynced,
		lastConfirmedBlock:  lastConfirmed,
		pendingEvents:       make(map[uint64][]*ProcessedEvent),
		checkpointInterval:  100,
		lastCheckpoint:      0,
	}, nil
}

func (ei *EnhancedIndexer) Start(ctx context.Context) error {
	ei.ctx, ei.cancel = context.WithCancel(ctx)
	defer ei.cancel()

	log.Println("Starting ENHANCED indexer with reorg protection...")
	log.Printf("Configuration:")
	log.Printf("  Block confirmations: %d", ei.config.BlockConfirmations)
	log.Printf("  Reorg check interval: %d seconds", ei.config.ReorgCheckInterval)
	log.Printf("  Start block: %d", ei.config.StartBlock)

	ei.validator.Start(ei.ctx)

	go ei.startReorgDetectionLoop(ei.ctx)

	go ei.startConfirmationProcessor(ei.ctx)

	go ei.startCheckpointCreator(ei.ctx)

	if err := ei.performInitialSync(ei.ctx); err != nil {
		log.Printf("Initial sync warning: %v", err)
	}

	if err := ei.startEventSubscription(ei.ctx); err != nil {
		return fmt.Errorf("event subscription failed: %v", err)
	}

	return nil
}

func (ei *EnhancedIndexer) startReorgDetectionLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(ei.config.ReorgCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Reorg detection loop shutting down...")
			return
		case <-ticker.C:
			ei.checkAndHandleReorg()
		}
	}
}

func (ei *EnhancedIndexer) checkAndHandleReorg() {
	analysis, err := ei.validator.performFullChainValidation()
	if err != nil {
		log.Printf("Chain validation error: %v", err)
		return
	}

	if analysis == nil {
		return
	}

	log.Printf("FORK DETECTED - initiating rollback procedure")

	if err := ei.rollbackMgr.HandleFork(analysis); err != nil {
		log.Printf("CRITICAL: Fork handling failed: %v", err)
		return
	}

	ei.eventMutex.Lock()
	for blockNum := range ei.pendingEvents {
		if blockNum > analysis.LastCommonBlock {
			delete(ei.pendingEvents, blockNum)
		}
	}
	ei.eventMutex.Unlock()

	ei.lastSyncedBlock = analysis.LastCommonBlock
	ei.lastConfirmedBlock = analysis.LastCommonBlock

	database.UpdateSyncState(ei.lastSyncedBlock, ei.lastConfirmedBlock)

	log.Printf("Starting resync from block %d", analysis.LastCommonBlock+1)
	go ei.resyncFromBlock(analysis.LastCommonBlock + 1)
}

func (ei *EnhancedIndexer) startConfirmationProcessor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentBlock, err := ei.validator.GetClient().BlockNumber(ctx)
			if err != nil {
				continue
			}

			if currentBlock <= ei.config.BlockConfirmations {
				continue
			}

			confirmedThreshold := currentBlock - ei.config.BlockConfirmations
			if confirmedThreshold > ei.lastConfirmedBlock {
				ei.confirmBlocksUpTo(confirmedThreshold)
				ei.lastConfirmedBlock = confirmedThreshold
				database.UpdateSyncState(ei.lastSyncedBlock, ei.lastConfirmedBlock)
			}
		}
	}
}

func (ei *EnhancedIndexer) confirmBlocksUpTo(threshold uint64) {
	ei.eventMutex.Lock()
	defer ei.eventMutex.Unlock()

	blocksToConfirm := make([]uint64, 0)
	for blockNum := range ei.pendingEvents {
		if blockNum <= threshold {
			blocksToConfirm = append(blocksToConfirm, blockNum)
		}
	}

	for _, blockNum := range blocksToConfirm {
		events := ei.pendingEvents[blockNum]
		if len(events) == 0 {
			delete(ei.pendingEvents, blockNum)
			continue
		}

		log.Printf("Confirming %d events from block %d", len(events), blockNum)

		for _, event := range events {
			if event.IsRolledBack {
				continue
			}
			ei.saveConfirmedEvent(event)
		}

		block, err := ei.validator.GetClient().BlockByNumber(
			ei.ctx,
			new(big.Int).SetUint64(blockNum),
		)
		if err == nil {
			chainBlock := &models.ChainBlock{
				BlockNumber:      blockNum,
				BlockHash:        block.Hash().Hex(),
				ParentHash:       block.ParentHash().Hex(),
				Timestamp:        block.Time(),
				TransactionCount: len(block.Transactions()),
				ValidationStatus: models.ValidationValidated,
				IsCanonical:      true,
				EventCount:       len(events),
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			database.DB.Create(chainBlock)
		}

		delete(ei.pendingEvents, blockNum)
	}
}

func (ei *EnhancedIndexer) saveConfirmedEvent(event *ProcessedEvent) {
	productState := &models.ProductState{
		ProductHash:     event.ProductHash,
		CurrentHash:     event.CurrentHash,
		PreviousHash:    event.PreviousHash,
		Status:          event.Status,
		StatusText:      event.Status.String(),
		Operator:        event.Operator,
		Timestamp:       event.Timestamp,
		BlockNumber:     event.BlockNumber,
		TransactionHash: event.TransactionHash,
		EventIndex:      event.EventIndex,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	var existing models.ProductState
	err := database.DB.Where("current_hash = ?", productState.CurrentHash).First(&existing).Error
	if err == nil {
		log.Printf("Event already confirmed: %s", productState.CurrentHash)
		return
	}

	if err != gorm.ErrRecordNotFound {
		log.Printf("Error checking existing event: %v", err)
		return
	}

	if err := database.DB.Create(productState).Error; err != nil {
		log.Printf("Failed to save confirmed event: %v", err)
		return
	}

	log.Printf("Event confirmed and saved: block=%d, status=%s, hash=%s",
		event.BlockNumber, event.Status.String(), event.CurrentHash)

	event.IsProcessed = true
}

func (ei *EnhancedIndexer) startCheckpointCreator(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ei.lastConfirmedBlock-ei.lastCheckpoint >= ei.checkpointInterval {
				ei.createCheckpoint()
			}
		}
	}
}

func (ei *EnhancedIndexer) createCheckpoint() {
	blockNum := ei.lastConfirmedBlock

	block, err := ei.validator.GetClient().BlockByNumber(
		ei.ctx,
		new(big.Int).SetUint64(blockNum),
	)
	if err != nil {
		log.Printf("Failed to get block for checkpoint: %v", err)
		return
	}

	var eventCount int64
	database.DB.Model(&models.ProductState{}).Count(&eventCount)

	if err := ei.rollbackMgr.CreateCheckpoint(
		blockNum,
		block.Hash().Hex(),
		block.ParentHash().Hex(),
		eventCount,
	); err != nil {
		log.Printf("Failed to create checkpoint: %v", err)
		return
	}

	ei.lastCheckpoint = blockNum
	log.Printf("Checkpoint created at block %d", blockNum)
}

func (ei *EnhancedIndexer) performInitialSync(ctx context.Context) error {
	log.Println("Performing initial sync...")

	currentBlock, err := ei.validator.GetClient().BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %v", err)
	}

	if ei.lastSyncedBlock >= currentBlock {
		log.Println("No initial sync needed")
		return nil
	}

	log.Printf("Syncing from block %d to %d", ei.lastSyncedBlock+1, currentBlock)

	eventSignature := ei.validator.GetContractABI().Events["ProductStateChanged"].ID
	batchSize := uint64(1000)

	for fromBlock := ei.lastSyncedBlock + 1; fromBlock <= currentBlock; fromBlock += batchSize {
		toBlock := fromBlock + batchSize - 1
		if toBlock > currentBlock {
			toBlock = currentBlock
		}

		log.Printf("Syncing blocks %d to %d...", fromBlock, toBlock)

		query := ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(fromBlock),
			ToBlock:   new(big.Int).SetUint64(toBlock),
			Addresses: []common.Address{ei.validator.GetContractAddress()},
			Topics:    [][]common.Hash{{eventSignature}},
		}

		logs, err := ei.validator.GetClient().FilterLogs(ctx, query)
		if err != nil {
			log.Printf("Failed to filter logs for blocks %d-%d: %v", fromBlock, toBlock, err)
			continue
		}

		log.Printf("Found %d events in blocks %d-%d", len(logs), fromBlock, toBlock)

		for _, logEntry := range logs {
			if logEntry.Removed {
				continue
			}

			event, err := ei.validator.parseEvent(logEntry)
			if err != nil {
				log.Printf("Failed to parse event: %v", err)
				continue
			}

			if currentBlock-logEntry.BlockNumber >= ei.config.BlockConfirmations {
				ei.saveConfirmedEvent(event)
			} else {
				ei.eventMutex.Lock()
				ei.pendingEvents[logEntry.BlockNumber] = append(
					ei.pendingEvents[logEntry.BlockNumber],
					event,
				)
				ei.eventMutex.Unlock()
			}
		}

		if toBlock > ei.lastSyncedBlock {
			ei.lastSyncedBlock = toBlock
			database.UpdateSyncState(ei.lastSyncedBlock, ei.lastConfirmedBlock)
		}
	}

	log.Println("Initial sync completed")
	return nil
}

func (ei *EnhancedIndexer) resyncFromBlock(startBlock uint64) error {
	log.Printf("Resyncing from block %d", startBlock)

	currentBlock, err := ei.validator.GetClient().BlockNumber(ei.ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %v", err)
	}

	if startBlock > currentBlock {
		return nil
	}

	eventSignature := ei.validator.GetContractABI().Events["ProductStateChanged"].ID

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(startBlock),
		ToBlock:   new(big.Int).SetUint64(currentBlock),
		Addresses: []common.Address{ei.validator.GetContractAddress()},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	logs, err := ei.validator.GetClient().FilterLogs(ei.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to filter logs: %v", err)
	}

	log.Printf("Found %d events during resync", len(logs))

	for _, logEntry := range logs {
		if logEntry.Removed {
			continue
		}

		event, err := ei.validator.parseEvent(logEntry)
		if err != nil {
			log.Printf("Failed to parse event during resync: %v", err)
			continue
		}

		if currentBlock-logEntry.BlockNumber >= ei.config.BlockConfirmations {
			ei.saveConfirmedEvent(event)
		} else {
			ei.eventMutex.Lock()
			ei.pendingEvents[logEntry.BlockNumber] = append(
				ei.pendingEvents[logEntry.BlockNumber],
				event,
			)
			ei.eventMutex.Unlock()
		}
	}

	ei.lastSyncedBlock = currentBlock
	if currentBlock > ei.config.BlockConfirmations {
		ei.lastConfirmedBlock = currentBlock - ei.config.BlockConfirmations
	}
	database.UpdateSyncState(ei.lastSyncedBlock, ei.lastConfirmedBlock)

	log.Printf("Resync completed, now synced to block %d", currentBlock)
	return nil
}

func (ei *EnhancedIndexer) startEventSubscription(ctx context.Context) error {
	eventSignature := ei.validator.GetContractABI().Events["ProductStateChanged"].ID

	query := ethereum.FilterQuery{
		Addresses: []common.Address{ei.validator.GetContractAddress()},
		Topics:    [][]common.Hash{{eventSignature}},
	}

	logsChan := make(chan types.Log)

	sub, err := ei.validator.GetClient().SubscribeFilterLogs(ctx, query, logsChan)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	log.Println("Event subscription started, listening for new events...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Event subscription shutting down...")
			return nil
		case err := <-sub.Err():
			log.Printf("Subscription error: %v", err)
			return err
		case logEntry := <-logsChan:
			ei.handleNewEvent(logEntry)
		}
	}
}

func (ei *EnhancedIndexer) handleNewEvent(logEntry types.Log) {
	if logEntry.Removed {
		log.Printf("Received REMOVED event - marking for potential rollback")
		ei.handleRemovedEvent(logEntry)
		return
	}

	event, err := ei.validator.parseEvent(logEntry)
	if err != nil {
		log.Printf("Failed to parse new event: %v", err)
		return
	}

	currentBlock, err := ei.validator.GetClient().BlockNumber(ei.ctx)
	if err != nil {
		currentBlock = logEntry.BlockNumber
	}

	if currentBlock-logEntry.BlockNumber >= ei.config.BlockConfirmations {
		ei.saveConfirmedEvent(event)
	} else {
		ei.eventMutex.Lock()
		ei.pendingEvents[logEntry.BlockNumber] = append(
			ei.pendingEvents[logEntry.BlockNumber],
			event,
		)
		ei.eventMutex.Unlock()
		log.Printf("New event cached (pending %d confirmations): block=%d, tx=%s",
			ei.config.BlockConfirmations, logEntry.BlockNumber, logEntry.TxHash.Hex())
	}

	if logEntry.BlockNumber > ei.lastSyncedBlock {
		ei.lastSyncedBlock = logEntry.BlockNumber
		database.UpdateSyncState(ei.lastSyncedBlock, ei.lastConfirmedBlock)
	}
}

func (ei *EnhancedIndexer) handleRemovedEvent(logEntry types.Log) {
	log.Printf("Handling REMOVED event from block %d, tx=%s",
		logEntry.BlockNumber, logEntry.TxHash.Hex())

	ei.eventMutex.Lock()
	defer ei.eventMutex.Unlock()

	events, exists := ei.pendingEvents[logEntry.BlockNumber]
	if exists {
		for i, event := range events {
			if event.TransactionHash == logEntry.TxHash.Hex() &&
				event.EventIndex == logEntry.Index {
				events[i].IsRolledBack = true
				log.Printf("Marked pending event as rolled back: %s", event.CurrentHash)
			}
		}
	}

	txHash := logEntry.TxHash.Hex()
	result := database.DB.Where(
		"transaction_hash = ? AND event_index = ?",
		txHash, logEntry.Index,
	).Delete(&models.ProductState{})

	if result.RowsAffected > 0 {
		log.Printf("Deleted %d confirmed events that were rolled back", result.RowsAffected)
	}
}

func (ei *EnhancedIndexer) GetSyncStatus() map[string]interface{} {
	ei.eventMutex.RLock()
	defer ei.eventMutex.RUnlock()

	pendingBlocks := len(ei.pendingEvents)
	pendingEvents := 0
	for _, events := range ei.pendingEvents {
		pendingEvents += len(events)
	}

	return map[string]interface{}{
		"last_synced_block":     ei.lastSyncedBlock,
		"last_confirmed_block":  ei.lastConfirmedBlock,
		"pending_blocks":        pendingBlocks,
		"pending_events":        pendingEvents,
		"block_confirmations":   ei.config.BlockConfirmations,
		"last_checkpoint":       ei.lastCheckpoint,
		"checkpoint_interval":   ei.checkpointInterval,
	}
}

func (ei *EnhancedIndexer) RollbackToCheckpoint(checkpointNum uint64) error {
	return ei.rollbackMgr.RollbackToCheckpoint(checkpointNum)
}

func (ei *EnhancedIndexer) VerifyIntegrity(startBlock, endBlock uint64) (bool, error) {
	return ei.rollbackMgr.VerifyChainIntegrity(startBlock, endBlock)
}

func hexEncodeBytes32(b [32]byte) string {
	return "0x" + hex.EncodeToString(b[:])
}
