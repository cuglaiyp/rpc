package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// 首先定义服务发现的结构体

type GeeRegistryDiscovery struct {
	*MultiServersDiscovery               // 继承
	registry               string        // 注册中心地址，会从这个地址更新可用服务
	timeout                time.Duration // 服务列表的过期时间
	lastUpdate             time.Time     // 上一次更新时间从注册中心拉取服务的时间
}

const defaultUpdateTimeout = time.Second * 10

// NewGeeRegistryDiscovery 同样来一个构造函数
func NewGeeRegistryDiscovery(registryAddr string, timeout time.Duration) *GeeRegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	return &GeeRegistryDiscovery{
		MultiServersDiscovery: NewMultiServerDiscovery([]string{}),
		registry:              registryAddr,
		timeout:               timeout,
	}
}

// 重写服务发现接口的方法

func (g *GeeRegistryDiscovery) Refresh() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastUpdate.Add(g.timeout).After(time.Now()) { // 服务列表还没过期，不用更新
		return nil
	}
	log.Println("rpc registry: refresh provides from registry", g.registry)
	resp, err := http.Get(g.registry) // 向注册中心发送 GET 请求，以拉取可用服务列表
	if err != nil {
		log.Println("rpc registry refresh err:", err)
		return err
	}
	serverStr := resp.Header.Get("X-Geerpc-Servers") // 从响应头中获取服务列表
	servers := strings.Split(serverStr, ",")
	g.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		g.servers = append(g.servers, strings.TrimSpace(server))
	}
	g.lastUpdate = time.Now() // 更新 上次更新 时间
	return nil
}

func (g *GeeRegistryDiscovery) Update(servers []string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.servers = servers
	g.lastUpdate = time.Now()
	return nil
}

func (g *GeeRegistryDiscovery) Get(mode SelectMode, serviceMethod ...string) (string, error) {
	if err := g.Refresh(); err != nil { // Get 前，刷新一下服务列表
		return "", err
	}
	return g.MultiServersDiscovery.Get(mode) // 直接调用父类方法即可
}

func (g *GeeRegistryDiscovery) GetAll() ([]string, error) {
	if err := g.Refresh(); err != nil {
		return nil, err
	}
	return g.MultiServersDiscovery.GetAll()
}
