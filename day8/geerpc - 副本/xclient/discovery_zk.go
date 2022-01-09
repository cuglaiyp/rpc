package xclient

import (
	"github.com/go-zookeeper/zk"
	"log"
	"time"
)

const (
	root     = "/geerpc"   // zookeeper 的根结点
	providers = "/providers" // 二级结点
)

type ZkRegistryDiscovery struct {
	*MultiServersDiscovery // 继承
	// registry               string        // 注册中心地址，会从这个地址更新可用服务
	conn       *zk.Conn      // 我们自己的注册中心是 Http 服务，而 Zookeeper 就维护一个连接
	timeout    time.Duration // 服务列表的过期时间
	lastUpdate time.Time     // 上一次更新时间从注册中心拉取服务的时间
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
	// 建立服务的根结点，保证所有服务都注册在这个根结点上
	z := &ZkRegistryDiscovery{
		MultiServersDiscovery: NewMultiServerDiscovery([]string{}),
		conn:                  conn,
		timeout:               timeout,
	}
	_, _, event, err := conn.ChildrenW(root + providers)
	go func(e <-chan zk.Event) {
		<-e
		z.refreshFromZk()

	}(event)
	return z
}

// 重写服务发现接口的方法

func (g *ZkRegistryDiscovery) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastUpdate.Add(g.timeout).After(time.Now()) { // 服务列表还没过期，不用更新
		return nil
	}
	log.Println("rpc registry: refresh provides from registry")
	// resp, err := http.Get(g.registry) // 向注册中心发送 GET 请求，以拉取可用服务列表

	return nil
}

func (g *ZkRegistryDiscovery) refreshFromZk() error {
	servers, _, err := g.conn.Children(root + providers)
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	g.servers = servers
	g.lastUpdate = time.Now() // 更新 上次更新 时间
	return nil
}

func (g *ZkRegistryDiscovery) Update(servers []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.servers = servers
	g.lastUpdate = time.Now()
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
