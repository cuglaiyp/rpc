package xclient

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// 前五天实现了服务端、客户端、服务注册、超时控制、支持 Http 协议
// 今天来完成多服务的负载均衡

// 多服务的负载均衡首先得要有服务发现，我们先来实现一个简单的服务发现模块

// Discovery 定义服务发现的通用接口
type Discovery interface {
	Refresh() error                      // 从远程结点更新服务
	Update(servers []string)             // 手动更新服务
	Get(mode SelectMode) (string, error) // 根据负载均衡策略选择服务
	GetAll() ([]string, error)           // 返回所有服务
}

// SelectMode Discovery 的 Get 方法需要根据负载均衡策略来选择服务
type SelectMode int

// 定义几种常见的负载均衡策略常量
const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

// 有了 Discovery 接口之后，我们可以来实现一个简单的实现类

// MultiServersDiscovery 手工维护 servers 的注册中心
type MultiServersDiscovery struct {
	r       *rand.Rand   // RandomSelect 策略用来随机的字段
	mu      sync.RWMutex // 同步锁
	servers []string     // 维护的服务
	index   int          // 用来记录被选择的服务的下标
}

func NewMultiServerDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
		servers: servers,
		mu: sync.RWMutex{}, // 由于 Go 存在默认值，所以也可以不用显示的构造
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

func (m MultiServersDiscovery) Refresh() error {
	panic("implement me")
}

func (m MultiServersDiscovery) Update(servers []string) {
	panic("implement me")
}

func (m MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	panic("implement me")
}

func (m MultiServersDiscovery) GetAll() ([]string, error) {
	panic("implement me")
}

var _ Discovery = (*MultiServersDiscovery)(nil)
