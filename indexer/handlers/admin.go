package handlers

import (
	"net/http"
	"strconv"

	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/gin-gonic/gin"
)

var globalIndexer IndexerStatus

type IndexerStatus interface {
	GetSyncStatus() map[string]interface{}
}

func SetIndexerStatus(idx IndexerStatus) {
	globalIndexer = idx
}

func GetSyncStatus(c *gin.Context) {
	if globalIndexer == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error:   "INDEXER_NOT_AVAILABLE",
			Message: "Indexer status not available",
		})
		return
	}

	status := globalIndexer.GetSyncStatus()
	c.JSON(http.StatusOK, status)
}

func GetReorgEvents(c *gin.Context) {
	limit := 100
	if l, ok := c.GetQuery("limit"); ok {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var events []models.ReorgEvent
	if err := database.DB.
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to query reorg events",
		})
		return
	}

	c.JSON(http.StatusOK, events)
}

func GetBlockRecords(c *gin.Context) {
	limit := 50
	if l, ok := c.GetQuery("limit"); ok {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var blocks []models.BlockRecord
	if err := database.DB.
		Order("block_number DESC").
		Limit(limit).
		Find(&blocks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to query block records",
		})
		return
	}

	c.JSON(http.StatusOK, blocks)
}
