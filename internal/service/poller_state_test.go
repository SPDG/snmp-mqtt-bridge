package service

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"snmp-mqtt-bridge/internal/domain"
)

func TestUpdateStateSubscribersDoNotRaceOnValues(t *testing.T) {
	poller := &PollerService{
		states: make(map[string]*domain.DeviceState),
	}
	mutatorCh := poller.Subscribe()
	marshalCh := poller.Subscribe()

	values := map[string]interface{}{
		"Voltage":          230.0,
		"Total Current":    0.2,
		"Outlet 1 Current": 0.2,
		"Outlet 1 Voltage": 0.0,
	}

	for i := 0; i < 50; i++ {
		poller.updateState("pdu-1", values, true, nil)

		mutatorEvent := <-mutatorCh
		marshalEvent := <-marshalCh

		if valuesAreAliases(mutatorEvent.Values, marshalEvent.Values) {
			t.Fatal("subscribers share one Values map")
		}
		stored := poller.GetDeviceState("pdu-1")
		if valuesAreAliases(mutatorEvent.Values, stored.Values) || valuesAreAliases(marshalEvent.Values, stored.Values) {
			t.Fatal("subscriber received the live device Values map")
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start
			mutateValuesLikeCalculatedPower(mutatorEvent.Values)
		}()

		go func() {
			defer wg.Done()
			<-start
			for n := 0; n < 20; n++ {
				if _, err := json.Marshal(marshalEvent); err != nil {
					t.Errorf("json.Marshal: %v", err)
				}
			}
		}()

		close(start)
		wg.Wait()

		if _, ok := marshalEvent.Values["Active Power"]; ok {
			t.Fatal("power written by one subscriber leaked into the other")
		}
		if _, ok := stored.Values["Active Power"]; ok {
			t.Fatal("power written by a subscriber leaked into stored state")
		}
	}
}

func TestGetAllDeviceStatesSnapshotDoesNotRaceWithUpdate(t *testing.T) {
	poller := &PollerService{
		states: make(map[string]*domain.DeviceState),
	}
	poller.updateState("pdu-1", map[string]interface{}{
		"Voltage":       230.0,
		"Total Current": 0.2,
	}, true, nil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			poller.updateState("pdu-1", map[string]interface{}{
				"Voltage":          float64(220 + i%10),
				"Total Current":    0.2,
				"Outlet 1 Current": 0.2,
			}, true, []string{"poll"})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			states := poller.GetAllDeviceStates()
			if _, err := json.Marshal(states); err != nil {
				t.Errorf("json.Marshal states: %v", err)
			}
			state := poller.GetDeviceState("pdu-1")
			if _, err := json.Marshal(state); err != nil {
				t.Errorf("json.Marshal state: %v", err)
			}
		}
	}()

	wg.Wait()
}

// mutateValuesLikeCalculatedPower writes the same keys calculatePowerIfNeeded adds.
func mutateValuesLikeCalculatedPower(values map[string]interface{}) {
	voltage := 230.0
	current := 0.2
	if _, ok := values["Active Power"]; !ok {
		values["Active Power"] = voltage * current
	}
	for name := range values {
		if name != "Outlet 1 Current" {
			continue
		}
		for n := 0; n < 20; n++ {
			values["Outlet "+strconv.Itoa(n)+" Power"] = voltage * current
		}
	}
}

func valuesAreAliases(a, b map[string]interface{}) bool {
	const probe = "__alias_probe__"
	a[probe] = true
	_, shared := b[probe]
	delete(a, probe)
	return shared
}
