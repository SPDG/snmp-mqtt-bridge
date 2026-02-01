package handler

import (
	"net/http"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// DeviceHandler handles device-related HTTP requests
type DeviceHandler struct {
	deviceService *service.DeviceService
	pollerService *service.PollerService
}

// NewDeviceHandler creates a new device handler
func NewDeviceHandler(deviceService *service.DeviceService, pollerService *service.PollerService) *DeviceHandler {
	return &DeviceHandler{
		deviceService: deviceService,
		pollerService: pollerService,
	}
}

// List returns all devices
func (h *DeviceHandler) List(c *gin.Context) {
	devices, err := h.deviceService.GetAll(c.Request.Context())
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondOK(c, devices)
}

// Get returns a device by ID
func (h *DeviceHandler) Get(c *gin.Context) {
	id := c.Param("id")

	device, err := h.deviceService.GetByID(c.Request.Context(), id)
	if err != nil {
		RespondNotFound(c, "Device not found")
		return
	}

	RespondOK(c, device)
}

// Create creates a new device
func (h *DeviceHandler) Create(c *gin.Context) {
	var req domain.DeviceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	device, err := h.deviceService.Create(c.Request.Context(), &req)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	// Notify poller about new device
	if h.pollerService != nil && device.Enabled {
		h.pollerService.AddDevice(device)
	}

	RespondCreated(c, device)
}

// Update updates an existing device
func (h *DeviceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.DeviceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	device, err := h.deviceService.Update(c.Request.Context(), id, &req)
	if err != nil {
		RespondNotFound(c, "Device not found")
		return
	}

	// Notify poller about device update
	if h.pollerService != nil {
		h.pollerService.UpdateDevice(device)
	}

	RespondOK(c, device)
}

// Delete deletes a device
func (h *DeviceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.deviceService.Delete(c.Request.Context(), id); err != nil {
		RespondNotFound(c, "Device not found")
		return
	}

	// Notify poller to remove device
	if h.pollerService != nil {
		h.pollerService.RemoveDevice(id)
	}

	c.JSON(http.StatusNoContent, nil)
}

// TestConnection tests SNMP connection to an existing device
func (h *DeviceHandler) TestConnection(c *gin.Context) {
	id := c.Param("id")

	device, err := h.deviceService.GetByID(c.Request.Context(), id)
	if err != nil {
		RespondNotFound(c, "Device not found")
		return
	}

	req := &domain.TestConnectionRequest{
		IPAddress:   device.IPAddress,
		Port:        device.Port,
		Community:   device.Community,
		SNMPVersion: device.SNMPVersion,
	}

	result, err := h.deviceService.TestConnection(c.Request.Context(), req)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondOK(c, result)
}

// TestNewConnection tests SNMP connection with provided parameters
func (h *DeviceHandler) TestNewConnection(c *gin.Context) {
	var req domain.TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	result, err := h.deviceService.TestConnection(c.Request.Context(), &req)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondOK(c, result)
}

// GetState returns the current state of a device
func (h *DeviceHandler) GetState(c *gin.Context) {
	id := c.Param("id")

	if h.pollerService == nil {
		RespondInternalError(c, "Poller service not available")
		return
	}

	state := h.pollerService.GetDeviceState(id)
	if state == nil {
		RespondNotFound(c, "Device state not available")
		return
	}

	RespondOK(c, state)
}
