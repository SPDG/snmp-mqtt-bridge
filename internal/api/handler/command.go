package handler

import (
	"fmt"

	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// CommandHandler handles device command requests
type CommandHandler struct {
	snmpService   *service.SNMPService
	pollerService *service.PollerService
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(snmpService *service.SNMPService, pollerService *service.PollerService) *CommandHandler {
	return &CommandHandler{
		snmpService:   snmpService,
		pollerService: pollerService,
	}
}

// SetValueRequest represents a request to set an SNMP value
type SetValueRequest struct {
	OID   string      `json:"oid" binding:"required"`
	Value interface{} `json:"value" binding:"required"`
}

// SetValue sets an SNMP value on a device
func (h *CommandHandler) SetValue(c *gin.Context) {
	deviceID := c.Param("id")

	var req SetValueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, req.OID, req.Value); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll to reflect the change
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": "Value set successfully",
	})
}

// GetValue gets an SNMP value from a device
func (h *CommandHandler) GetValue(c *gin.Context) {
	deviceID := c.Param("id")
	oid := c.Query("oid")

	if oid == "" {
		RespondBadRequest(c, "OID is required")
		return
	}

	value, err := h.snmpService.GetValue(c.Request.Context(), deviceID, oid)
	if err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	RespondOK(c, gin.H{
		"oid":   oid,
		"value": value,
	})
}

// SwitchSourceRequest for switching ATS source
type SwitchSourceRequest struct {
	Source int `json:"source" binding:"required,oneof=1 2"` // 1 = Source A, 2 = Source B
}

// SwitchSource switches the ATS preferred source
func (h *CommandHandler) SwitchSource(c *gin.Context) {
	deviceID := c.Param("id")

	var req SwitchSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// ATS switch command OID
	switchOID := ".1.3.6.1.4.1.318.1.1.8.4.2.0"

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, switchOID, req.Source); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	sourceName := "Source A"
	if req.Source == 2 {
		sourceName = "Source B"
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": "Switched to " + sourceName,
	})
}

// SetSourceNameRequest for setting source name
type SetSourceNameRequest struct {
	Source int    `json:"source" binding:"required,oneof=1 2"` // 1 = Source A, 2 = Source B
	Name   string `json:"name" binding:"required"`
}

// SetSourceName sets the name of an ATS source
func (h *CommandHandler) SetSourceName(c *gin.Context) {
	deviceID := c.Param("id")

	var req SetSourceNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// Source name OIDs
	// Source A: .1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.1
	// Source B: .1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.2
	var nameOID string
	if req.Source == 1 {
		nameOID = ".1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.1"
	} else {
		nameOID = ".1.3.6.1.4.1.318.1.1.8.5.3.2.1.6.2"
	}

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, nameOID, req.Name); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": "Source name updated",
	})
}

// PDU Outlet Control

// SetOutletStateRequest for setting PDU outlet state
type SetOutletStateRequest struct {
	Outlet int    `json:"outlet" binding:"required,min=1,max=48"` // Outlet number (1-48)
	State  string `json:"state" binding:"required,oneof=on off"`  // "on" or "off"
}

// SetOutletState turns a PDU outlet on or off
func (h *CommandHandler) SetOutletState(c *gin.Context) {
	deviceID := c.Param("id")

	var req SetOutletStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// PDU outlet control OID: .1.3.6.1.4.1.318.1.1.12.3.3.1.1.4.<outlet>
	// Values: 1 = immediateOn, 2 = immediateOff, 3 = immediateReboot
	controlOID := fmt.Sprintf(".1.3.6.1.4.1.318.1.1.12.3.3.1.1.4.%d", req.Outlet)

	var value int
	if req.State == "on" {
		value = 1 // immediateOn
	} else {
		value = 2 // immediateOff
	}

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, controlOID, value); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("Outlet %d turned %s", req.Outlet, req.State),
	})
}

// SetOutletNameRequest for setting PDU outlet name
type SetOutletNameRequest struct {
	Outlet int    `json:"outlet" binding:"required,min=1,max=48"` // Outlet number (1-48)
	Name   string `json:"name" binding:"required"`
}

// SetOutletName sets the name of a PDU outlet
func (h *CommandHandler) SetOutletName(c *gin.Context) {
	deviceID := c.Param("id")

	var req SetOutletNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// PDU outlet name OID (config table - writable): .1.3.6.1.4.1.318.1.1.12.3.4.1.1.2.<outlet>
	nameOID := fmt.Sprintf(".1.3.6.1.4.1.318.1.1.12.3.4.1.1.2.%d", req.Outlet)

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, nameOID, req.Name); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("Outlet %d name set to '%s'", req.Outlet, req.Name),
	})
}

// RebootOutletRequest for rebooting a PDU outlet
type RebootOutletRequest struct {
	Outlet int `json:"outlet" binding:"required,min=1,max=48"` // Outlet number (1-48)
}

// RebootOutlet reboots a PDU outlet (turns off then on)
func (h *CommandHandler) RebootOutlet(c *gin.Context) {
	deviceID := c.Param("id")

	var req RebootOutletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// PDU outlet control OID with value 3 = immediateReboot
	controlOID := fmt.Sprintf(".1.3.6.1.4.1.318.1.1.12.3.3.1.1.4.%d", req.Outlet)

	if err := h.snmpService.SetValue(c.Request.Context(), deviceID, controlOID, 3); err != nil {
		RespondError(c, 500, err.Error())
		return
	}

	// Trigger immediate poll
	if h.pollerService != nil {
		h.pollerService.TriggerPoll(deviceID)
	}

	RespondOK(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("Outlet %d rebooting", req.Outlet),
	})
}
