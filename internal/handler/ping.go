package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go-musthave-metrics/internal/config/db"
)

type PingHandler struct {
	db *db.DB
}

func NewPingHandler(db *db.DB) *PingHandler {
	return &PingHandler{db: db}
}

func (h *PingHandler) PingHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database connection failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}