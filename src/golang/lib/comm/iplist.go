package comm

import "strings"

// WsURLFromIplistEntry 将 /im/iplist 返回的 list 项转为 WebSocket URL。
// 支持 host:port（legacy）或完整 ws:// / wss:// URL（K8s Ingress 方案 A）。
func WsURLFromIplistEntry(entry string) string {
	entry = strings.TrimSpace(entry)
	if strings.HasPrefix(entry, "ws://") || strings.HasPrefix(entry, "wss://") {
		return entry
	}
	return "ws://" + entry + "/im"
}
