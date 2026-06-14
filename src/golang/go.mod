module beehive-im

go 1.20

require (
	github.com/apache/thrift v0.12.0
	github.com/astaxie/beego v1.12.3
	github.com/garyburd/redigo v1.6.4
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang/protobuf v1.4.2
	github.com/gorilla/websocket v1.4.2
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/prometheus/client_golang v1.7.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.10.0 // indirect
	github.com/prometheus/procfs v0.1.3 // indirect
	github.com/shiena/ansicolor v0.0.0-20151119151921-a422bbe96644 // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	golang.org/x/text v0.3.0 // indirect
	google.golang.org/protobuf v1.23.0 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

// 与 seq_svr_thrift.go 生成代码匹配的旧版 Thrift（无 context API）
replace github.com/apache/thrift => ./vendor/git.apache.org/thrift.git
