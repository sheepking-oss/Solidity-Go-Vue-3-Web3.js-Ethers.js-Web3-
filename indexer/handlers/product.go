package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"supply-chain-indexer/database"
	"supply-chain-indexer/models"

	"github.com/gin-gonic/gin"
)

type TraceResponse struct {
	SerialNumber string             `json:"serial_number"`
	ProductHash  string             `json:"product_hash"`
	Timeline     []models.ProductState `json:"timeline"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func GetProductTrace(c *gin.Context) {
	serialNumber := c.Query("serial_number")

	if serialNumber == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Serial number is required",
		})
		return
	}

	serialNumber = strings.TrimSpace(serialNumber)

	productHash := hashSerialNumber(serialNumber)

	var states []models.ProductState
	if err := database.DB.Where("product_hash = ?", productHash).
		Order("timestamp ASC").
		Find(&states).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to query product history",
		})
		return
	}

	if len(states) == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "No history found for this product",
		})
		return
	}

	for i := range states {
		states[i].SerialNumber = serialNumber
	}

	response := TraceResponse{
		SerialNumber: serialNumber,
		ProductHash:  productHash,
		Timeline:     states,
	}

	c.JSON(http.StatusOK, response)
}

func GetProductByHash(c *gin.Context) {
	productHash := c.Param("hash")

	if productHash == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAMETER",
			Message: "Product hash is required",
		})
		return
	}

	var states []models.ProductState
	if err := database.DB.Where("product_hash = ?", productHash).
		Order("timestamp ASC").
		Find(&states).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to query product history",
		})
		return
	}

	if len(states) == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "No history found for this product",
		})
		return
	}

	c.JSON(http.StatusOK, states)
}

func GetAllProducts(c *gin.Context) {
	limit := 100
	offset := 0

	var states []models.ProductState
	if err := database.DB.Distinct("ON (product_hash)").
		Order("product_hash, timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&states).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "DATABASE_ERROR",
			Message: "Failed to query products",
		})
		return
	}

	type ProductSummary struct {
		ProductHash string `json:"product_hash"`
		LatestStatus string `json:"latest_status"`
		LatestTime   int64  `json:"latest_time"`
	}

	var summaries []ProductSummary
	for _, state := range states {
		summaries = append(summaries, ProductSummary{
			ProductHash:  state.ProductHash,
			LatestStatus: state.StatusText,
			LatestTime:   state.Timestamp,
		})
	}

	c.JSON(http.StatusOK, summaries)
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "supply-chain-indexer",
	})
}

func hashSerialNumber(serialNumber string) string {
	hash := sha256.Sum256([]byte(serialNumber))
	return "0x" + hex.EncodeToString(hash[:])
}
