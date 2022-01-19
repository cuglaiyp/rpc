module geerpc

go 1.16

require (
	github.com/go-zookeeper/zk v1.0.2
	go.etcd.io/etcd/client/v3 v3.5.1
	tlv v0.0.0
)

replace (
	tlv => ../tlv
)

