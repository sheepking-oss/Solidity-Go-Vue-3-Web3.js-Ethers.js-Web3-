package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type ChainForkType string

const (
	ForkTypeReorg       ChainForkType = "reorg"
	ForkTypeOrphan      ChainForkType = "orphan"
	ForkTypeTemporary   ChainForkType = "temporary"
)

type BlockValidationStatus string

const (
	ValidationPending    BlockValidationStatus = "pending"
	ValidationValidated  BlockValidationStatus = "validated"
	ValidationOrphaned   BlockValidationStatus = "orphaned"
	ValidationForked     BlockValidationStatus = "forked"
)

type ChainBlock struct {
	ID                  uint                   `gorm:"primaryKey" json:"id"`
	BlockNumber         uint64                 `gorm:"index;uniqueIndex:idx_block_number_hash" json:"block_number"`
	BlockHash           string                 `gorm:"index;uniqueIndex:idx_block_number_hash;size:66" json:"block_hash"`
	ParentHash          string                 `gorm:"index;not null;size:66" json:"parent_hash"`
	Timestamp           uint64                 `json:"timestamp"`
	GasUsed             uint64                 `json:"gas_used"`
	TransactionCount    int                    `json:"transaction_count"`
	ValidationStatus    BlockValidationStatus  `gorm:"index;not null;default:'pending'" json:"validation_status"`
	IsCanonical         bool                   `gorm:"index;default:true" json:"is_canonical"`
	ForkDepth           int                    `gorm:"default:0" json:"fork_depth"`
	EventCount          int                    `gorm:"default:0" json:"event_count"`
	RawData             []byte                 `gorm:"type:jsonb" json:"-"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	DeletedAt           gorm.DeletedAt         `gorm:"index" json:"-"`
}

type ForkEvent struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	ForkType            ChainForkType  `gorm:"index;not null" json:"fork_type"`
	LastCommonBlock     uint64         `gorm:"index;not null" json:"last_common_block"`
	LastCommonHash      string         `gorm:"not null;size:66" json:"last_common_hash"`
	ForkedFromBlock     uint64         `gorm:"index;not null" json:"forked_from_block"`
	OldChainBlockCount  int            `gorm:"not null" json:"old_chain_block_count"`
	NewChainBlockCount  int            `gorm:"not null" json:"new_chain_block_count"`
	AffectedEvents      int64          `gorm:"not null;default:0" json:"affected_events"`
	RollbackCompleted   bool           `gorm:"default:false" json:"rollback_completed"`
	ResyncCompleted     bool           `gorm:"default:false" json:"resync_completed"`
	ErrorMsg            string         `gorm:"size:1024" json:"error_msg"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type SyncCheckpoint struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	CheckpointNumber    uint64         `gorm:"uniqueIndex;not null" json:"checkpoint_number"`
	CheckpointHash      string         `gorm:"uniqueIndex;not null;size:66" json:"checkpoint_hash"`
	ParentHash          string         `gorm:"not null;size:66" json:"parent_hash"`
	EventCountAtCheckpoint int64       `gorm:"not null;default:0" json:"event_count_at_checkpoint"`
	IsVerified          bool           `gorm:"default:false" json:"is_verified"`
	CreatedAt           time.Time      `json:"created_at"`
}

type OrphanedBlock struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	BlockNumber         uint64         `gorm:"index;not null" json:"block_number"`
	BlockHash           string         `gorm:"uniqueIndex;not null;size:66" json:"block_hash"`
	ParentHash          string         `gorm:"not null;size:66" json:"parent_hash"`
	DetectedAtBlock     uint64         `gorm:"not null" json:"detected_at_block"`
	Reason              string         `gorm:"size:255" json:"reason"`
	EventCount          int            `gorm:"default:0" json:"event_count"`
	EventsRolledBack    bool           `gorm:"default:false" json:"events_rolled_back"`
	KeptForAnalysis     bool           `gorm:"default:false" json:"kept_for_analysis"`
	CreatedAt           time.Time      `json:"created_at"`
}

func (cb *ChainBlock) BeforeCreate(tx *gorm.DB) error {
	if cb.ValidationStatus == "" {
		cb.ValidationStatus = ValidationPending
	}
	return nil
}

func (cb *ChainBlock) SetRawData(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	cb.RawData = bytes
	return nil
}

func (cb *ChainBlock) GetRawData(target interface{}) error {
	return json.Unmarshal(cb.RawData, target)
}

type ChainLink struct {
	BlockNumber uint64
	BlockHash   string
	ParentHash  string
	IsValid     bool
}

type ReorgAnalysis struct {
	LastCommonBlock uint64
	LastCommonHash  string
	OldChainBlocks  []ChainLink
	NewChainBlocks  []ChainLink
	ForkType        ChainForkType
	ForkDepth       int
}
