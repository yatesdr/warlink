package plcman

import (
	"fmt"
	"testing"
	"time"

	"github.com/yatesdr/plcio/driver"

	"warlink/config"
)

type stubDriver struct{}

func (d stubDriver) Connect() error                                       { return nil }
func (d stubDriver) Close() error                                         { return nil }
func (d stubDriver) IsConnected() bool                                    { return true }
func (d stubDriver) Family() driver.PLCFamily                             { return driver.FamilyLogix }
func (d stubDriver) ConnectionMode() string                               { return "test" }
func (d stubDriver) GetDeviceInfo() (*driver.DeviceInfo, error)           { return nil, nil }
func (d stubDriver) SupportsDiscovery() bool                              { return false }
func (d stubDriver) AllTags() ([]driver.TagInfo, error)                   { return nil, nil }
func (d stubDriver) Programs() ([]string, error)                          { return nil, nil }
func (d stubDriver) Read([]driver.TagRequest) ([]*driver.TagValue, error) { return nil, nil }
func (d stubDriver) Write(string, interface{}) error                      { return nil }
func (d stubDriver) Keepalive() error                                     { return nil }
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
