package comm

import (
	"net/http"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsOnce syncOnce

	chatroomRoomChatAckTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "beehive_chatroom_room_chat_ack_total",
		Help: "ROOM-CHAT ACK count",
	})
	chatroomBroadcastQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "beehive_chatroom_broadcast_queue_depth",
		Help: "len(room_broadcast_chan)",
	})
	chatroomKafkaProduceTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "beehive_chatroom_kafka_produce_total",
		Help: "Kafka produce success count",
	})
	chatroomKafkaProduceErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "beehive_chatroom_kafka_produce_errors",
		Help: "Kafka produce error count",
	})
	wsConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "beehive_ws_connections",
		Help: "Active websocket sessions",
	})
	iplistStaticTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "beehive_iplist_static_total",
		Help: "Static iplist hits",
	})
	roomfanoutConsumeTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "beehive_roomfanout_consume_total",
		Help: "roomfanout consumed messages",
	})
)

type syncOnce struct {
	done uint32
}

func (o *syncOnce) Do(fn func()) {
	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
		fn()
	}
}

func metricsEnabled() bool {
	return os.Getenv("BEEHIVE_METRICS") == "1"
}

func registerMetrics() {
	prometheus.MustRegister(
		chatroomRoomChatAckTotal,
		chatroomBroadcastQueueDepth,
		chatroomKafkaProduceTotal,
		chatroomKafkaProduceErrors,
		wsConnections,
		iplistStaticTotal,
		roomfanoutConsumeTotal,
	)
}

// StartMetricsServer 在 BEEHIVE_METRICS=1 时暴露 /metrics。
func StartMetricsServer(defaultPort int) {
	if !metricsEnabled() {
		return
	}
	metricsOnce.Do(registerMetrics)

	port := defaultPort
	if v := os.Getenv("BEEHIVE_METRICS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	go func() {
		_ = http.ListenAndServe(":"+strconv.Itoa(port), promhttp.Handler())
	}()
}

func IncChatroomRoomChatAck() {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		chatroomRoomChatAckTotal.Inc()
	}
}

func SetChatroomBroadcastQueueDepth(n int) {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		chatroomBroadcastQueueDepth.Set(float64(n))
	}
}

func IncChatroomKafkaProduceOK() {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		chatroomKafkaProduceTotal.Inc()
	}
}

func IncChatroomKafkaProduceErr() {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		chatroomKafkaProduceErrors.Inc()
	}
}

func SetWsConnections(n int) {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		wsConnections.Set(float64(n))
	}
}

func IncIplistStaticHit() {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		iplistStaticTotal.Inc()
	}
}

func IncRoomfanoutConsume() {
	if metricsEnabled() {
		metricsOnce.Do(registerMetrics)
		roomfanoutConsumeTotal.Inc()
	}
}
