package plcman

import (
	"fmt"
	"testing"
	"time"

	"github.com/yatesdr/plcio/driver"

	"warlink/config"
)

type stubDriver struct {
	connected  bool
	readValues []*driver.TagValue
	readErr    error
}

func (d stubDriver) Connect() error                             { return nil }
func (d stubDriver) Close() error                               { return nil }
func (d stubDriver) IsConnected() bool                          { return d.connected }
func (d stubDriver) Family() driver.PLCFamily                   { return driver.FamilyLogix }
func (d stubDriver) ConnectionMode() string                     { return "test" }
func (d stubDriver) GetDeviceInfo() (*driver.DeviceInfo, error) { return nil, nil }
func (d stubDriver) SupportsDiscovery() bool                    { return false }
func (d stubDriver) AllTags() ([]driver.TagInfo, error)         { return nil, nil }
func (d stubDriver) Programs() ([]string, error)                { return nil, nil }
func (d stubDriver) Read([]driver.TagRequest) ([]*driver.TagValue, error) {
	return d.readValues, d.readErr
}
func (d stubDriver) Write(string, interface{}) error { return nil }
func (d stubDriver) Keepalive() error                { return nil }
func (d stubDriver) IsConnectionError(err error) bool {
	return err != nil && (err.Error() == "not connected" || err.Error() == "SendUnitDataTransaction: not connected")
}

func TestAllDriverValuesConnectionError(t *testing.T) {
	drv := stubDriver{}

	err := allDriverValuesConnectionError([]*driver.TagValue{
		{Name: "A", Error: fmt.Errorf("SendUnitDataTransaction: not connected")},
		{Name: "B", Error: fmt.Errorf("not connected")},
	}, drv)
	if err == nil {
		t.Fatal("expected connection error when all values fail with connection errors")
	}
}

func TestAllDriverValuesConnectionErrorMixedResults(t *testing.T) {
	drv := stubDriver{}

	err := allDriverValuesConnectionError([]*driver.TagValue{
		{Name: "A", Error: fmt.Errorf("SendUnitDataTransaction: not connected")},
		{Name: "B", Value: int32(1)},
	}, drv)
	if err != nil {
		t.Fatalf("expected nil for mixed success/failure results, got %v", err)
	}
}

func TestStaleThreshold(t *testing.T) {
	m := NewManager(time.Second)

	// Fast poll rate: floor of MinStaleThreshold applies (3*250ms < 15s).
	if got := m.staleThreshold(&config.PLCConfig{PollRate: 250 * time.Millisecond}); got != MinStaleThreshold {
		t.Errorf("fast poll: expected floor %v, got %v", MinStaleThreshold, got)
	}

	// Slow poll rate: 3*pollRate dominates the floor.
	if got := m.staleThreshold(&config.PLCConfig{PollRate: 10 * time.Second}); got != 30*time.Second {
		t.Errorf("slow poll: expected 30s, got %v", got)
	}

	// Unset per-PLC rate falls back to the manager default (1s) -> floor.
	if got := m.staleThreshold(&config.PLCConfig{}); got != MinStaleThreshold {
		t.Errorf("default poll: expected floor %v, got %v", MinStaleThreshold, got)
	}
}

func TestGetValuesSnapshotIncludesUpdateTime(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 17, 12, 30, 0, 0, time.UTC)
	plc := &ManagedPLC{
		Values: map[string]*TagValue{
			"Counter": {Name: "Counter"},
		},
		LastPoll:        updatedAt.Add(time.Minute),
		valuesUpdatedAt: updatedAt,
	}

	values, gotUpdatedAt := plc.GetValuesSnapshot()
	if gotUpdatedAt != updatedAt {
		t.Fatalf("expected update time %v, got %v", updatedAt, gotUpdatedAt)
	}
	if _, ok := values["Counter"]; !ok {
		t.Fatal("expected Counter in values snapshot")
	}

	delete(values, "Counter")
	if _, ok := plc.Values["Counter"]; !ok {
		t.Fatal("mutating snapshot map changed the PLC cache")
	}
}

func TestFailedPollPreservesCacheTimestamp(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 15, 2, 45, 0, 0, time.UTC)
	plc := &ManagedPLC{
		Config: &config.PLCConfig{
			Name:    "SNF1",
			Enabled: false, // Avoid scheduling a reconnect goroutine in this unit test.
			Tags: []driver.TagSelection{
				{Name: "Counter", Enabled: true},
			},
		},
		Driver:          stubDriver{connected: true, readErr: fmt.Errorf("SendUnitDataTransaction: not connected")},
		Values:          map[string]*TagValue{"Counter": {Name: "Counter"}},
		Status:          StatusConnected,
		LastPoll:        updatedAt,
		valuesUpdatedAt: updatedAt,
	}

	newPLCWorker(plc, NewManager(time.Second), time.Second).poll()

	if plc.valuesUpdatedAt != updatedAt {
		t.Fatalf("failed poll advanced cache timestamp from %v to %v", updatedAt, plc.valuesUpdatedAt)
	}
	if plc.LastPoll != updatedAt {
		t.Fatalf("failed poll advanced LastPoll from %v to %v", updatedAt, plc.LastPoll)
	}
	if plc.Status != StatusError {
		t.Fatalf("expected failed poll to set status error, got %v", plc.Status)
	}
}

func TestSuccessfulPollAdvancesCacheTimestamp(t *testing.T) {
	updatedAt := time.Now().Add(-time.Hour)
	plc := &ManagedPLC{
		Config: &config.PLCConfig{
			Name:    "SNF1",
			Enabled: false,
			Tags: []driver.TagSelection{
				{Name: "Counter", Enabled: true},
			},
		},
		Driver: stubDriver{
			connected: true,
			readValues: []*driver.TagValue{
				{Name: "Counter", Value: int32(514), StableValue: int32(514)},
			},
		},
		Values:          make(map[string]*TagValue),
		Status:          StatusConnected,
		LastPoll:        updatedAt,
		valuesUpdatedAt: updatedAt,
	}

	newPLCWorker(plc, NewManager(time.Second), time.Second).poll()

	if !plc.valuesUpdatedAt.After(updatedAt) {
		t.Fatalf("successful poll did not advance cache timestamp: %v", plc.valuesUpdatedAt)
	}
	if plc.LastPoll != plc.valuesUpdatedAt {
		t.Fatalf("successful poll timestamps differ: LastPoll=%v valuesUpdatedAt=%v", plc.LastPoll, plc.valuesUpdatedAt)
	}
	if got := plc.Values["Counter"].GoValue(); got != int32(514) {
		t.Fatalf("expected cached counter 514, got %v", got)
	}
}
