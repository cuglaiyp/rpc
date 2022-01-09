package xclient

import "testing"

func TestNewZkRegistryDiscovery(t *testing.T) {
	zkRegistryDiscovery := NewZkRegistryDiscovery("localhost:2181", defaultUpdateTimeout)
	if zkRegistryDiscovery == nil {
		panic("connect failed")
	}
	err := zkRegistryDiscovery.Refresh()
	if err != nil {
		panic("connect failed")
	}
}
