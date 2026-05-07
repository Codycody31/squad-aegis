package plugin_manager

import "testing"

func TestConnectorInvokeDataSize(t *testing.T) {
	t.Parallel()

	if connectorInvokeDataSize(nil) != 0 {
		t.Fatalf("nil map size = %d", connectorInvokeDataSize(nil))
	}
	n := connectorInvokeDataSize(map[string]interface{}{"action": "ping"})
	if n < 10 || n > 512 {
		t.Fatalf("unexpected size %d", n)
	}
}
