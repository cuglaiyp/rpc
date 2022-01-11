package xclient

import (
	"geerpc/config"
	"github.com/go-zookeeper/zk"
	"log"
	"time"
)

type ZkRegistryDiscovery struct {
	*GeeRegistryDiscovery               // 继承这个，可以复用 Update 方法
	conn                  *zk.Conn      // 我们自己的注册中心是 Http 服务，而 Zookeeper 就维护一个连接
	timeout               time.Duration // 服务列表的过期时间
	lastUpdate            time.Time     // 上一次更新时间从注册中心拉取服务的时间
}

// NewZkRegistryDiscovery 同样来一个构造函数
func NewZkRegistryDiscovery(registryAddr string, timeout time.Duration) *ZkRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	conn, _, err := zk.Connect([]string{registryAddr}, timeout)
	if err != nil {
		log.Printf("rpc discovery_zk: cannot connect: %s: %s", registryAddr, err.Error())
		return nil
	}
	g := &ZkRegistryDiscovery{
		GeeRegistryDiscovery: NewGeeRegistryDiscovery(registryAddr, timeout),
		conn:                 conn,
		timeout:              timeout,
	}
	go g.watchProviders()
	return g
}

func (g *ZkRegistryDiscovery) watchProviders() {
	for {
		// 循环监听提供服务结点的子结点是否发生变化（因为这个监听是一次性的，使用之后就会失效，所以需要循环注册）
		_, _, event, err := g.conn.ChildrenW(config.ZkProviderPath)
		if err != nil {
			return
		}
		<-event
		// 子结点产生了变化，就从服务器拉取
		g.refreshFromZk()
	}
}

// 重写服务发现接口的方法

func (g *ZkRegistryDiscovery) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastUpdate.Add(g.timeout).After(time.Now()) { // 服务列表还没过期，不用更新
		return nil
	}
	log.Println("rpc discovery: refresh provides from registry")
	// resp, err := http.Get(g.registry) // 向注册中心发送 GET 请求，以拉取可用服务列表
	return g.refreshFromZk()
}

func (g *ZkRegistryDiscovery) refreshFromZk() error {
	servers, _, err := g.conn.Children(config.ZkProviderPath)
	if err != nil {
		log.Println("rpc discovery: refresh err:", err)
		return err
	}
	g.servers = servers
	g.lastUpdate = time.Now() // 更新 上次更新 时间
	// go g.watchProviders() // 监听的第二种方案：每次 refresh 的时候就启动一个协程监听 // 不好：因为一旦结点没有变化，而由于频繁的 Get 操作，会导致开启很多协程进行阻塞
	return nil
}

func (g *ZkRegistryDiscovery) Get(mode SelectMode, serviceMethod ...string) (string, error) {
	if err := g.Refresh(); err != nil { // Get 前，刷新一下服务列表
		return "", err
	}
	return g.MultiServersDiscovery.Get(mode, serviceMethod...) // 直接调用父类方法即可
}

func (g *ZkRegistryDiscovery) GetAll() ([]string, error) {
	if err := g.Refresh(); err != nil {
		return nil, err
	}
	return g.MultiServersDiscovery.GetAll()
}

func (g *ZkRegistryDiscovery) Close() error {
	g.conn.Close()
	return nil
}

var _ Discovery = (*ZkRegistryDiscovery)(nil)
