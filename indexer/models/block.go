package models

import (
	"time"

	"gorm.io/gorm"
)

type BlockStatus string

const (
	BlockStatusPending    BlockStatus = "pending"
	BlockStatusConfirmed  BlockStatus = "confirmed"
	BlockStatusOrphaned   BlockStatus = "orphaned"
)

type BlockRecord struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BlockNumber uint64         `gorm:"uniqueIndex;not null" json:"block_number"`
	BlockHash   string         `gorm:"uniqueIndex;not null;size:66" json:"block_hash"`
	ParentHash  string         `gorm:"not null;size:66" json:"parent_hash"`
	Status      BlockStatus    `gorm:"index;not null;default:'pending'" json:"status"`
	IsCanonical bool           `gorm:"index;default:true" json:"is_canonical"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type ReorgEvent struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	FromBlock      uint64         `gorm:"index;not null" json:"from_block"`
	ToBlock        uint64         `gorm:"index;not null" json:"to_block"`
	AffectedEvents int64          `gorm:"not null;default:0" json:"affected_events"`
	Reason         string         `gorm:"size:255" json:"reason"`
	CreatedAt      time.Time      `json:"created_at"`
}

type SyncState struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	LastSyncedBlock uint64         `gorm:"uniqueIndex;not null;default:0" json:"last_synced_block"`
	LastConfirmedBlock uint64      `gorm:"not null;default:0" json:"last_confirmed_block"`
	LockKey         string         `gorm:"uniqueIndex;not null;size:64" json:"lock_key"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type PendingEvent struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	BlockNumber     uint64         `gorm:"index;not null" json:"block_number"`
	BlockHash       string         `gorm:"not null;size:66" json:"block_hash"`
	EventData       []byte         `gorm:"not null" json:"event_data"`
	TransactionHash string         `gorm:"not null;size:66" json:"transaction_hash"`
	EventIndex      uint           `gorm:"not null" json:"event_index"`
	CreatedAt       time.Time      `json:"created_at"`
}

func (b *BlockRecord) BeforeCreate(tx *gorm.DB) error {
	if b.Status == "" {
		b.Status = BlockStatusPending
	}
	if !b.IsCanonical {
		b.IsCanonical = true
	}
	return nil
}
