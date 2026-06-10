package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/repository"
)

type PingHandler struct {
	storage *repository.PostgresStorage
}

func NewPingHandler(storage *repository.PostgresStorage) *PingHandler {
	return &PingHandler{storage: storage}
}

func (h *PingHandler) PingHandler(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "database not configured",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.storage.Ping(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database connection failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}