package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

func TestDeviceHandler_CreateRegistersEnabledDevice(t *testing.T) {
	repo := newFakeDeviceRepo()
	publisher := &fakeDevicePublisher{}
	handler := NewDeviceHandler(service.NewDeviceService(repo), nil, publisher)

	w := performDeviceRequest(http.MethodPost, "/devices", "/devices", handler.Create, map[string]interface{}{
		"name":         "New ATS",
		"ip_address":   "192.168.26.50",
		"community":    "private",
		"snmp_version": "v2c",
		"profile_id":   "apc-ats",
		"enabled":      true,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
	if len(publisher.registered) != 1 {
		t.Fatalf("expected one MQTT registration, got %d", len(publisher.registered))
	}
	if publisher.registered[0].Name != "New ATS" {
		t.Fatalf("expected registered device name New ATS, got %q", publisher.registered[0].Name)
	}
}

func TestDeviceHandler_CreateDoesNotRegisterDisabledDevice(t *testing.T) {
	repo := newFakeDeviceRepo()
	publisher := &fakeDevicePublisher{}
	handler := NewDeviceHandler(service.NewDeviceService(repo), nil, publisher)

	w := performDeviceRequest(http.MethodPost, "/devices", "/devices", handler.Create, map[string]interface{}{
		"name":         "Disabled ATS",
		"ip_address":   "192.168.26.51",
		"community":    "private",
		"snmp_version": "v2c",
		"profile_id":   "apc-ats",
		"enabled":      false,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
	if len(publisher.registered) != 0 {
		t.Fatalf("expected no MQTT registrations, got %d", len(publisher.registered))
	}
}

func TestDeviceHandler_UpdateRefreshesEnabledDeviceRegistration(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.devices["dev1"] = &domain.Device{
		ID:          "dev1",
		Name:        "Old ATS",
		IPAddress:   "192.168.26.52",
		Port:        161,
		Community:   "private",
		SNMPVersion: domain.SNMPv2c,
		ProfileID:   "apc-ats",
		Enabled:     true,
	}
	publisher := &fakeDevicePublisher{}
	handler := NewDeviceHandler(service.NewDeviceService(repo), nil, publisher)

	w := performDeviceRequest(http.MethodPut, "/devices/:id", "/devices/dev1", handler.Update, map[string]interface{}{
		"name": "Renamed ATS",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	if len(publisher.unregistered) != 1 || publisher.unregistered[0] != "dev1" {
		t.Fatalf("expected dev1 MQTT unregistration, got %#v", publisher.unregistered)
	}
	if len(publisher.registered) != 1 {
		t.Fatalf("expected one refreshed MQTT registration, got %d", len(publisher.registered))
	}
	if publisher.registered[0].Name != "Renamed ATS" {
		t.Fatalf("expected refreshed device name Renamed ATS, got %q", publisher.registered[0].Name)
	}
}

func TestDeviceHandler_UpdateUnregistersDisabledDevice(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.devices["dev1"] = &domain.Device{
		ID:          "dev1",
		Name:        "ATS",
		IPAddress:   "192.168.26.53",
		Port:        161,
		Community:   "private",
		SNMPVersion: domain.SNMPv2c,
		ProfileID:   "apc-ats",
		Enabled:     true,
	}
	publisher := &fakeDevicePublisher{}
	handler := NewDeviceHandler(service.NewDeviceService(repo), nil, publisher)

	w := performDeviceRequest(http.MethodPut, "/devices/:id", "/devices/dev1", handler.Update, map[string]interface{}{
		"enabled": false,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	if len(publisher.unregistered) != 1 || publisher.unregistered[0] != "dev1" {
		t.Fatalf("expected dev1 MQTT unregistration, got %#v", publisher.unregistered)
	}
	if len(publisher.registered) != 0 {
		t.Fatalf("expected no refreshed MQTT registration, got %d", len(publisher.registered))
	}
}

func TestDeviceHandler_DeleteUnregistersDevice(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.devices["dev1"] = &domain.Device{
		ID:          "dev1",
		Name:        "ATS",
		IPAddress:   "192.168.26.54",
		Port:        161,
		Community:   "private",
		SNMPVersion: domain.SNMPv2c,
		ProfileID:   "apc-ats",
		Enabled:     true,
	}
	publisher := &fakeDevicePublisher{}
	handler := NewDeviceHandler(service.NewDeviceService(repo), nil, publisher)

	w := performDeviceRequest(http.MethodDelete, "/devices/:id", "/devices/dev1", handler.Delete, nil)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}
	if len(publisher.unregistered) != 1 || publisher.unregistered[0] != "dev1" {
		t.Fatalf("expected dev1 MQTT unregistration, got %#v", publisher.unregistered)
	}
}

func performDeviceRequest(method, routePath, requestPath string, handlerFunc gin.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, routePath, handlerFunc)

	var requestBody bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&requestBody).Encode(body)
	}

	req := httptest.NewRequest(method, requestPath, &requestBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

type fakeDevicePublisher struct {
	registered   []*domain.Device
	unregistered []string
}

func (p *fakeDevicePublisher) RegisterDevice(device *domain.Device) error {
	copied := *device
	p.registered = append(p.registered, &copied)
	return nil
}

func (p *fakeDevicePublisher) UnregisterDevice(deviceID string) error {
	p.unregistered = append(p.unregistered, deviceID)
	return nil
}

type fakeDeviceRepo struct {
	devices map[string]*domain.Device
}

func newFakeDeviceRepo() *fakeDeviceRepo {
	return &fakeDeviceRepo{devices: make(map[string]*domain.Device)}
}

func (r *fakeDeviceRepo) Create(_ context.Context, device *domain.Device) error {
	copied := *device
	r.devices[device.ID] = &copied
	return nil
}

func (r *fakeDeviceRepo) GetByID(_ context.Context, id string) (*domain.Device, error) {
	device, ok := r.devices[id]
	if !ok {
		return nil, errFakeNotFound
	}
	copied := *device
	return &copied, nil
}

func (r *fakeDeviceRepo) GetAll(_ context.Context) ([]domain.Device, error) {
	devices := make([]domain.Device, 0, len(r.devices))
	for _, device := range r.devices {
		devices = append(devices, *device)
	}
	return devices, nil
}

func (r *fakeDeviceRepo) GetEnabled(_ context.Context) ([]domain.Device, error) {
	devices := make([]domain.Device, 0)
	for _, device := range r.devices {
		if device.Enabled {
			devices = append(devices, *device)
		}
	}
	return devices, nil
}

func (r *fakeDeviceRepo) Update(_ context.Context, device *domain.Device) error {
	if _, ok := r.devices[device.ID]; !ok {
		return errFakeNotFound
	}
	copied := *device
	r.devices[device.ID] = &copied
	return nil
}

func (r *fakeDeviceRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.devices[id]; !ok {
		return errFakeNotFound
	}
	delete(r.devices, id)
	return nil
}

func (r *fakeDeviceRepo) UpdateLastSeen(_ context.Context, id string) error {
	device, ok := r.devices[id]
	if !ok {
		return errFakeNotFound
	}
	now := time.Now()
	device.LastSeen = &now
	return nil
}

var errFakeNotFound = &fakeNotFoundError{}

type fakeNotFoundError struct{}

func (e *fakeNotFoundError) Error() string {
	return "not found"
}
