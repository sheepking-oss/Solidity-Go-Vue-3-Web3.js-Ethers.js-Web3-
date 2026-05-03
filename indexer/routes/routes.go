package routes

import (
	"supply-chain-indexer/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	router := gin.Default()

	router.Use(corsMiddleware())

	api := router.Group("/api")
	{
		api.GET("/health", handlers.HealthCheck)
		
		products := api.Group("/products")
		{
			products.GET("/trace", handlers.GetProductTrace)
			products.GET("/hash/:hash", handlers.GetProductByHash)
			products.GET("", handlers.GetAllProducts)
		}

		admin := api.Group("/admin")
		{
			admin.GET("/sync-status", handlers.GetSyncStatus)
			admin.GET("/fork-events", handlers.GetForkEvents)
			admin.GET("/reorg-events", handlers.GetReorgEvents)
			admin.GET("/orphaned-blocks", handlers.GetOrphanedBlocks)
			admin.GET("/canonical-blocks", handlers.GetCanonicalBlocks)
			admin.GET("/checkpoints", handlers.GetCheckpoints)
			admin.GET("/blocks", handlers.GetBlockRecords)
		}
	}

	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
