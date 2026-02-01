package handler

import (
	"net/http"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// ProfileHandler handles profile-related HTTP requests
type ProfileHandler struct {
	profileService *service.ProfileService
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: profileService}
}

// List returns all profiles
func (h *ProfileHandler) List(c *gin.Context) {
	profiles, err := h.profileService.GetAll(c.Request.Context())
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondOK(c, profiles)
}

// Get returns a profile by ID
func (h *ProfileHandler) Get(c *gin.Context) {
	id := c.Param("id")

	profile, err := h.profileService.GetByID(c.Request.Context(), id)
	if err != nil {
		RespondNotFound(c, "Profile not found")
		return
	}

	RespondOK(c, profile)
}

// Create creates a new profile
func (h *ProfileHandler) Create(c *gin.Context) {
	var profile domain.Profile
	if err := c.ShouldBindJSON(&profile); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.profileService.Create(c.Request.Context(), &profile); err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondCreated(c, profile)
}

// Update updates an existing profile
func (h *ProfileHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var profile domain.Profile
	if err := c.ShouldBindJSON(&profile); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	profile.ID = id

	if err := h.profileService.Update(c.Request.Context(), &profile); err != nil {
		RespondNotFound(c, "Profile not found")
		return
	}

	RespondOK(c, profile)
}

// Delete deletes a profile
func (h *ProfileHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.profileService.Delete(c.Request.Context(), id); err != nil {
		RespondNotFound(c, "Profile not found")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
