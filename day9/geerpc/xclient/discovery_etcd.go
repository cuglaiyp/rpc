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
	cancelWatch           func()           // 取消监听协程
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
	ctx, cancelFunc := context.WithCancel(context.Background())
	z.cancelWatch = cancelFunc
	go z.watchProviders(ctx)
	return z
}

func (g *EtcdRegistryDiscovery) watchProviders(ctx context.Context) {
	watchChan := clientv3.NewWatcher(g.client).Watch(context.TODO(), config.EtcdProviderPath, clientv3.WithPrefix())
	select {
	case <-watchChan:
		// 消耗掉 chan，别阻塞
		for _ = range watchChan {
			// 这里可以做得更精细，因为 etcd 会给出变化的 key，我们权且简单处理
			// 结点产生了变化，就从服务器拉取
		}
		g.refreshFromEtcd() // 全量更新一遍
	case <-ctx.Done():
	}

}

// 重写服务发现接口的方法
func (g *EtcdRegistryDiscovery) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastUpdate.Add(g.timeout).After(time.Now()) { // 服务列表还没过期，不用更新
		return nil
	}
	log.Println("rpc discovery: refresh providers from registry")
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
	g.cancelWatch()  // 先取消监听
	g.client.Close() // 然后关闭
	return nil
}

var _ Discovery = (*ZkRegistryDiscovery)(nil)
