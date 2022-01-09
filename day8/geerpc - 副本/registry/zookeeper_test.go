package registry

import (
	"testing"
	"time"
)

func TestZkClient_putServer(t *testing.T) {
	client := NewZkClient("localhost:2181", 5*time.Second)
	client.PutServer("localhost:9999")
}
