package database

import (
	"fmt"
	"log"

	"supply-chain-indexer/config"
	"supply-chain-indexer/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	log.Println("Successfully connected to database")

	err = DB.AutoMigrate(
		&models.ProductState{},
		&models.ChainBlock{},
		&models.ForkEvent{},
		&models.SyncState{},
		&models.SyncCheckpoint{},
		&models.OrphanedBlock{},
		&models.BlockRecord{},
		&models.ReorgEvent{},
		&models.PendingEvent{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}

	if err := initSyncState(cfg.StartBlock); err != nil {
		return fmt.Errorf("failed to initialize sync state: %v", err)
	}

	log.Println("Database migration completed")
	return nil
}

func initSyncState(startBlock uint64) error {
	var count int64
	if err := DB.Model(&models.SyncState{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		syncState := &models.SyncState{
			LastSyncedBlock:    startBlock,
			LastConfirmedBlock: startBlock,
			LockKey:            "supply-chain-indexer-lock",
		}
		return DB.Create(syncState).Error
	}

	return nil
}

func GetSyncState() (*models.SyncState, error) {
	var state models.SyncState
	if err := DB.First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func UpdateSyncState(syncedBlock, confirmedBlock uint64) error {
	return DB.Model(&models.SyncState{}).
		Where("1 = 1").
		Updates(map[string]interface{}{
			"last_synced_block":    syncedBlock,
			"last_confirmed_block": confirmedBlock,
		}).Error
}

func GetLatestCheckpoint() (*models.SyncCheckpoint, error) {
	var checkpoint models.SyncCheckpoint
	err := DB.
		Where("is_verified = ?", true).
		Order("checkpoint_number DESC").
		First(&checkpoint).Error
	if err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

func GetForkEvents(limit int) ([]models.ForkEvent, error) {
	var events []models.ForkEvent
	err := DB.
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func GetOrphanedBlocks(limit int) ([]models.OrphanedBlock, error) {
	var blocks []models.OrphanedBlock
	err := DB.
		Order("created_at DESC").
		Limit(limit).
		Find(&blocks).Error
	return blocks, err
}

func GetCanonicalBlocks(limit int) ([]models.ChainBlock, error) {
	var blocks []models.ChainBlock
	err := DB.
		Where("is_canonical = ?", true).
		Order("block_number DESC").
		Limit(limit).
		Find(&blocks).Error
	return blocks, err
}
