package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "video-service",
		})
	})
	
	// Video call endpoints
	r.POST("/v1/calls/initiate", func(c *gin.Context) {
		// TODO: Implement call initiation logic
		c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
	})
	
	r.POST("/v1/calls/:id/end", func(c *gin.Context) {
		// TODO: Implement call end logic
		c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
	})
	
	// WebSocket endpoint for WebRTC signaling
	r.GET("/v1/ws/signaling", func(c *gin.Context) {
		// TODO: Implement WebRTC signaling WebSocket
		c.JSON(http.StatusNotImplemented, gin.H{"message": "WebSocket signaling not implemented yet"})
	})
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	
	fmt.Printf("Video Service starting on port %s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start Video Service: %v", err)
	}
}
