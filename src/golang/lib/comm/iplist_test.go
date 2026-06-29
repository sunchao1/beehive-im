package comm

import "testing"

func TestWsURLFromIplistEntry(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"127.0.0.1:8002", "ws://127.0.0.1:8002/im"},
		{"ws://127.0.0.1:8002/im", "ws://127.0.0.1:8002/im"},
		{"wss://im.example.com/im", "wss://im.example.com/im"},
		{" 127.0.0.1:8003 ", "ws://127.0.0.1:8003/im"},
	}
	for _, c := range cases {
		if got := WsURLFromIplistEntry(c.in); got != c.want {
			t.Fatalf("WsURLFromIplistEntry(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
