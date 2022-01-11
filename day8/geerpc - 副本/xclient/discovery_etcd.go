package xclient

import (
	"context"
	"geerpc/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

type EtcdRegistryDiscovery struct {
	*GeeRegistryDiscovery                  // 继承
	client                *clientv3.Client // 我们自己的注册中心是 Http 服务，而 etcd 就维护一个连接
	timeout               time.Duration    // 服务列表的过期时间
	lastUpdate            time.Time        // 上一次更新时间从注册中心拉取服务的时间
}

// NewEtcdRegistryDiscovery 同样来一个构造函数
func NewEtcdRegistryDiscovery(registryAddr string, timeout time.Duration) *EtcdRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{registryAddr},
		DialTimeout: timeout,
	})
	if err != nil {
		log.Printf("rpc discovery_etcd: cannot connect: %s: %s", registryAddr, err.Error())
		return nil
	}
	z := &EtcdRegistryDiscovery{
		GeeRegistryDiscovery: NewGeeRegistryDiscovery(registryAddr, timeout),
		client:               client,
		timeout:              timeout,
	}
	z.watchProviders()
	return z
}

func (g *EtcdRegistryDiscovery) watchProviders() {
	// 循环监听提供服务结点的子结点是否发生变化（因为这个监听是一次性的，使用之后就会失效，所以需要循环注册）
	watchChan := g.client.Watch(context.Background(), config.EtcdProviderPath, clientv3.WithPrefix())
	<- watchChan
	// 子结点产生了变化，就从服务器拉取
	g.refreshFromEtcd()
}

// 重写服务发现接口的方法

func (g *EtcdRegistryDiscovery) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastUpdate.Add(g.timeout).After(time.Now()) { // 服务列表还没过期，不用更新
		return nil
	}
	log.Println("rpc discovery: refresh provides from registry")
	// resp, err := http.Get(g.registry) // 向注册中心发送 GET 请求，以拉取可用服务列表
	return g.refreshFromEtcd()
}

func (g *EtcdRegistryDiscovery) refreshFromEtcd() error {
	resp, err := g.client.Get(context.Background(), config.EtcdProviderPath, clientv3.WithPrefix())
	if err != nil {
		log.Println("rpc discovery: refresh err:", err)
		return err
	}
	g.servers = make([]string, 0, resp.Count)
	for i, _ := range resp.Kvs {
		g.servers = append(g.servers, string(resp.Kvs[i].Value))
	}
	g.lastUpdate = time.Now() // 更新 上次更新 时间
	return nil
}

func (g *EtcdRegistryDiscovery) Get(mode SelectMode, serviceMethod ...string) (string, error) {
	if err := g.Refresh(); err != nil { // Get 前，刷新一下服务列表
		return "", err
	}
	return g.MultiServersDiscovery.Get(mode, serviceMethod...) // 直接调用父类方法即可
}

func (g *EtcdRegistryDiscovery) GetAll() ([]string, error) {
	if err := g.Refresh(); err != nil {
		return nil, err
	}
	return g.MultiServersDiscovery.GetAll()
}

func (g *EtcdRegistryDiscovery) Close() error {
	g.client.Close()
	return nil
}

var _ Discovery = (*ZkRegistryDiscovery)(nil)
