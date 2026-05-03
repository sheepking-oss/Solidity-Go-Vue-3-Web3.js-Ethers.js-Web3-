package ethereum

import (
	"fmt"
	"log"
	"time"

	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"gorm.io/gorm"
)

type RollbackManager struct {
	validator *ChainValidator
}

func NewRollbackManager(validator *ChainValidator) *RollbackManager {
	return &RollbackManager{
		validator: validator,
	}
}

func (rm *RollbackManager) HandleFork(analysis *models.ReorgAnalysis) error {
	if analysis == nil || len(analysis.OldChainBlocks) == 0 {
		return nil
	}

	log.Printf("=== STARTING FORK HANDLING ===")
	log.Printf("Fork type: %s, depth: %d", analysis.ForkType, analysis.ForkDepth)
	log.Printf("Last common block: %d (%s)", analysis.LastCommonBlock, analysis.LastCommonHash)
	log.Printf("Old chain blocks: %d, New chain blocks: %d",
		len(analysis.OldChainBlocks), len(analysis.NewChainBlocks))

	forkEvent := &models.ForkEvent{
		ForkType:           analysis.ForkType,
		LastCommonBlock:    analysis.LastCommonBlock,
		LastCommonHash:     analysis.LastCommonHash,
		ForkedFromBlock:    analysis.OldChainBlocks[0].BlockNumber,
		OldChainBlockCount: len(analysis.OldChainBlocks),
		NewChainBlockCount: len(analysis.NewChainBlocks),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := database.DB.Create(forkEvent).Error; err != nil {
		log.Printf("Warning: failed to create fork event record: %v", err)
	}

	affectedEvents, err := rm.countAffectedEvents(analysis)
	if err != nil {
		log.Printf("Warning: failed to count affected events: %v", err)
	}
	forkEvent.AffectedEvents = affectedEvents

	if err := rm.performRollback(analysis, forkEvent); err != nil {
		forkEvent.ErrorMsg = err.Error()
		forkEvent.RollbackCompleted = false
		database.DB.Save(forkEvent)
		return fmt.Errorf("rollback failed: %v", err)
	}

	forkEvent.RollbackCompleted = true
	forkEvent.UpdatedAt = time.Now()
	database.DB.Save(forkEvent)

	log.Printf("=== FORK HANDLING COMPLETE ===")
	log.Printf("Rolled back %d events", affectedEvents)

	return nil
}

func (rm *RollbackManager) countAffectedEvents(analysis *models.ReorgAnalysis) (int64, error) {
	if len(analysis.OldChainBlocks) == 0 {
		return 0, nil
	}

	minBlock := analysis.OldChainBlocks[0].BlockNumber
	maxBlock := analysis.OldChainBlocks[0].BlockNumber

	for _, link := range analysis.OldChainBlocks {
		if link.BlockNumber < minBlock {
			minBlock = link.BlockNumber
		}
		if link.BlockNumber > maxBlock {
			maxBlock = link.BlockNumber
		}
	}

	var count int64
	err := database.DB.Model(&models.ProductState{}).
		Where("block_number >= ? AND block_number <= ?", minBlock, maxBlock).
		Count(&count).Error

	return count, err
}

func (rm *RollbackManager) performRollback(analysis *models.ReorgAnalysis, forkEvent *models.ForkEvent) error {
	if len(analysis.OldChainBlocks) == 0 {
		return nil
	}

	oldBlockHashes := make(map[string]bool)
	oldBlockNumbers := make(map[uint64]bool)

	for _, link := range analysis.OldChainBlocks {
		oldBlockHashes[link.BlockHash] = true
		oldBlockNumbers[link.BlockNumber] = true
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := rm.recordOrphanedBlocks(tx, analysis); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record orphaned blocks: %v", err)
	}

	var productStates []models.ProductState
	if err := tx.Where("block_number IN ?", getBlockNumbers(analysis.OldChainBlocks)).
		Find(&productStates).Error; err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return fmt.Errorf("failed to find product states to rollback: %v", err)
	}

	if len(productStates) > 0 {
		log.Printf("Found %d product states to rollback", len(productStates))

		if err := tx.Where("block_number IN ?", getBlockNumbers(analysis.OldChainBlocks)).
			Delete(&models.ProductState{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete product states: %v", err)
		}
	}

	if err := tx.Model(&models.ChainBlock{}).
		Where("block_number IN ?", getBlockNumbers(analysis.OldChainBlocks)).
		Updates(map[string]interface{}{
			"validation_status": models.ValidationOrphaned,
			"is_canonical":      false,
			"updated_at":        time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update block records: %v", err)
	}

	for _, link := range analysis.OldChainBlocks {
		orphaned := &models.OrphanedBlock{
			BlockNumber:      link.BlockNumber,
			BlockHash:        link.BlockHash,
			ParentHash:       link.ParentHash,
			DetectedAtBlock:  analysis.LastCommonBlock + uint64(len(analysis.NewChainBlocks)),
			Reason:           string(analysis.ForkType),
			EventsRolledBack: true,
			CreatedAt:        time.Now(),
		}

		if err := tx.Create(orphaned).Error; err != nil {
			log.Printf("Warning: failed to create orphaned block record: %v", err)
		}
	}

	if analysis.LastCommonBlock > 0 {
		if err := tx.Model(&models.SyncState{}).
			Where("1 = 1").
			Updates(map[string]interface{}{
				"last_synced_block":    analysis.LastCommonBlock,
				"last_confirmed_block": analysis.LastCommonBlock,
				"updated_at":           time.Now(),
			}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update sync state: %v", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully rolled back %d blocks and associated events", len(analysis.OldChainBlocks))

	return nil
}

func (rm *RollbackManager) recordOrphanedBlocks(tx *gorm.DB, analysis *models.ReorgAnalysis) error {
	for _, link := range analysis.OldChainBlocks {
		var existing models.ChainBlock
		err := tx.Where("block_hash = ?", link.BlockHash).First(&existing).Error
		if err == nil {
			existing.ValidationStatus = models.ValidationOrphaned
			existing.IsCanonical = false
			existing.ForkDepth = analysis.ForkDepth
			existing.UpdatedAt = time.Now()
			tx.Save(&existing)
		} else if err == gorm.ErrRecordNotFound {
			newBlock := &models.ChainBlock{
				BlockNumber:      link.BlockNumber,
				BlockHash:        link.BlockHash,
				ParentHash:       link.ParentHash,
				ValidationStatus: models.ValidationOrphaned,
				IsCanonical:      false,
				ForkDepth:        analysis.ForkDepth,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			tx.Create(newBlock)
		}
	}
	return nil
}

func (rm *RollbackManager) CreateCheckpoint(blockNum uint64, blockHash string, parentHash string, eventCount int64) error {
	checkpoint := &models.SyncCheckpoint{
		CheckpointNumber:       blockNum,
		CheckpointHash:         blockHash,
		ParentHash:             parentHash,
		EventCountAtCheckpoint: eventCount,
		IsVerified:             true,
		CreatedAt:              time.Now(),
	}

	return database.DB.Create(checkpoint).Error
}

func (rm *RollbackManager) GetLatestCheckpoint() (*models.SyncCheckpoint, error) {
	var checkpoint models.SyncCheckpoint
	err := database.DB.
		Where("is_verified = ?", true).
		Order("checkpoint_number DESC").
		First(&checkpoint).Error
	if err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

func (rm *RollbackManager) RollbackToCheckpoint(checkpointNum uint64) error {
	var checkpoint models.SyncCheckpoint
	if err := database.DB.
		Where("checkpoint_number = ? AND is_verified = ?", checkpointNum, true).
		First(&checkpoint).Error; err != nil {
		return fmt.Errorf("checkpoint not found: %v", err)
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %v", tx.Error)
	}

	if err := tx.Where("block_number > ?", checkpoint.CheckpointNumber).
		Delete(&models.ProductState{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to rollback product states: %v", err)
	}

	if err := tx.Model(&models.ChainBlock{}).
		Where("block_number > ?", checkpoint.CheckpointNumber).
		Updates(map[string]interface{}{
			"validation_status": models.ValidationForked,
			"is_canonical":      false,
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update block records: %v", err)
	}

	if err := tx.Model(&models.SyncState{}).
		Where("1 = 1").
		Updates(map[string]interface{}{
			"last_synced_block":    checkpoint.CheckpointNumber,
			"last_confirmed_block": checkpoint.CheckpointNumber,
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update sync state: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Successfully rolled back to checkpoint %d", checkpoint.CheckpointNumber)
	return nil
}

func (rm *RollbackManager) VerifyChainIntegrity(startBlock, endBlock uint64) (bool, error) {
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		var block models.ChainBlock
		if err := database.DB.
			Where("block_number = ? AND is_canonical = ?", blockNum, true).
			First(&block).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return false, err
		}

		chainBlock, err := rm.validator.GetClient().BlockByNumber(
			rm.validator.ctx,
			uint64ToBigInt(blockNum),
		)
		if err != nil {
			log.Printf("Warning: failed to get chain block %d: %v", blockNum, err)
			continue
		}

		if chainBlock.Hash().Hex() != block.BlockHash {
			log.Printf("INTEGRITY FAILURE: Block %d hash mismatch", blockNum)
			return false, nil
		}
	}

	return true, nil
}

func getBlockNumbers(blocks []models.ChainLink) []uint64 {
	numbers := make([]uint64, len(blocks))
	for i, block := range blocks {
		numbers[i] = block.BlockNumber
	}
	return numbers
}

func uint64ToBigInt(n uint64) *big.Int {
	return new(big.Int).SetUint64(n)
}
