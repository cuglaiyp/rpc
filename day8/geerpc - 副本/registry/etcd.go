package registry

import (
	"context"
	"geerpc/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
	"log"
	"time"
)

type EtcdClient struct {
	client  *clientv3.Client
	timeout time.Duration // 连接超时间，也是服务断开后，关联键被删除的时间
}

func NewEtcdClient(addr []string, timeout time.Duration) *EtcdClient {
	if timeout == 0 {
		timeout = defaultTimeout - time.Duration(1)*time.Minute
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   addr,
		DialTimeout: timeout,
	})
	if err != nil {
		log.Printf("rpc etcd: cannot connect to %s: err: %s", addr, err)
		return nil
	}
	return &EtcdClient{client: client, timeout: timeout}
}

func (e *EtcdClient) PutServer(server string) error {
	return e.Put(config.EtcdProviderPath+"/"+server, server)
}

func (e *EtcdClient) Put(key, value string) error {
	lease := clientv3.NewLease(e.client)
	leaseGrantResponse, err := lease.Grant(context.Background(), int64(e.timeout/time.Second))
	if err != nil {
		return nil
	}
	_, err = e.client.Put(context.TODO(), key, value, clientv3.WithLease(leaseGrantResponse.ID))
	if err != nil {
		return err
	}
	// 保持心跳（也就是对 key 自动续租）
	keepAliveResponse, err := lease.KeepAlive(context.TODO(), leaseGrantResponse.ID)
	if err != nil {
		return err
	}
	go leaseKeepAlive(keepAliveResponse)
	return nil
}

func leaseKeepAlive(response <-chan *clientv3.LeaseKeepAliveResponse) {
	for true {
		select {
		case ret := <-response:
			if ret == nil {
				return
			}
		}
	}
}

var _ io.Closer = (*EtcdClient)(nil)

func (e *EtcdClient) Close() error {
	e.client.Close()
	return nil
}
