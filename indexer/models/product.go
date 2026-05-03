package models

import (
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

type ProductStatus int

const (
	StatusManufactured ProductStatus = iota
	StatusShipped
	StatusDelivered
)

func (s ProductStatus) String() string {
	switch s {
	case StatusManufactured:
		return "Manufactured"
	case StatusShipped:
		return "Shipped"
	case StatusDelivered:
		return "Delivered"
	default:
		return "Unknown"
	}
}

type ProductState struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ProductHash     string         `gorm:"index;uniqueIndex:idx_product_hash_current_hash" json:"product_hash"`
	SerialNumber    string         `gorm:"index" json:"serial_number"`
	CurrentHash     string         `gorm:"uniqueIndex:idx_product_hash_current_hash" json:"current_hash"`
	PreviousHash    string         `json:"previous_hash"`
	Status          ProductStatus  `gorm:"index" json:"status"`
	StatusText      string         `json:"status_text"`
	Operator        string         `json:"operator"`
	Timestamp       int64          `json:"timestamp"`
	BlockNumber     uint64         `json:"block_number"`
	TransactionHash string         `json:"transaction_hash"`
	EventIndex      uint           `json:"event_index"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (p *ProductState) BeforeCreate(tx *gorm.DB) error {
	p.StatusText = p.Status.String()
	return nil
}

func (p *ProductState) BeforeUpdate(tx *gorm.DB) error {
	p.StatusText = p.Status.String()
	return nil
}

func Bytes32ToHex(b [32]byte) string {
	return "0x" + hex.EncodeToString(b[:])
}
