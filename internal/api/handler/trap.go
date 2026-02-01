package handler

import (
	"net/http"
	"strconv"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// TrapHandler handles trap-related HTTP requests
type TrapHandler struct {
	trapService *service.TrapLogService
}

// NewTrapHandler creates a new trap handler
func NewTrapHandler(trapService *service.TrapLogService) *TrapHandler {
	return &TrapHandler{trapService: trapService}
}

// List returns all trap logs with pagination
func (h *TrapHandler) List(c *gin.Context) {
	filter := domain.TrapFilter{
		DeviceID: c.Query("device_id"),
		Severity: domain.TrapSeverity(c.Query("severity")),
		Limit:    50,
		Offset:   0,
	}

	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	}

	if offset, err := strconv.Atoi(c.Query("offset")); err == nil && offset >= 0 {
		filter.Offset = offset
	}

	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartTime = &t
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndTime = &t
		}
	}

	traps, total, err := h.trapService.GetAll(c.Request.Context(), filter)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondWithMeta(c, traps, total, filter.Limit, filter.Offset)
}

// Get returns a trap by ID
func (h *TrapHandler) Get(c *gin.Context) {
	id := c.Param("id")

	trap, err := h.trapService.GetByID(c.Request.Context(), id)
	if err != nil {
		RespondNotFound(c, "Trap not found")
		return
	}

	RespondOK(c, trap)
}

// Cleanup deletes old trap logs
func (h *TrapHandler) Cleanup(c *gin.Context) {
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	deleted, err := h.trapService.DeleteOlderThan(c.Request.Context(), days)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
	})
}
