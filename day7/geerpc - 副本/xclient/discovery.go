package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// 前五天实现了服务端、客户端、服务注册、超时控制、支持 Http 协议
// 今天来完成多服务的负载均衡

/*
1. 在 codec.go 中
    - 我们抽象了编解码器的接口 Codec，设计好了 Header（Body 目前还不知道存放什么东西，先放着）
    - 定义了编解码器的种类，以及存放编解码器构造函数的容器
2. 在 gob.go 中
    - 我们定义了一个具体的编解码器：GobCodec，实现了 Codec
    - 实现了该编解码器的构造函数
3. 在 server.go 中
    - 我们约定好了 rpc 报文的标识字段，封装成了 Option，并制定用 Json 解析
    - 我们定义好了 server，并完成了初步功能
      |- 1. 解析 Option
		|- 2. 判断魔数，根据编解码协议，解码内容
		|- 3. 根据内容处理请求，并响应
      在请求处理的过程中，因为一个请求报文中可以装多个 Header 和 Body 对，所以使用了并发处理，并且因为这些报文对在同一个连接中，
      那么需要在所有 goroutine 结束后，才能关闭连接，所以使用了 waitGroup；又因为 Header 和 Body 是成对的，所以在局部写入的
      时候，使用了 mutex 来保证它们的原子性。
    - 添加了默认 server 并且，添加了包名启动服务的函数
    - 注意 154: var h *codec.Header 的坑，这里的 h 是 nil，所以解析会报错。推荐用 h := &codec.Header{}
4. 在 client.go 中
	- 我们将一次调用封装成一个 Call，并且在发送请求的时候，将这个 Call 的字段设置分成 Header 和 Body 写到服务端。
      所以 Client 需要持有 Call 的集合和一个可复用的 Header
    - 因为涉及到 Call 的操作，所以我们在 Client 结构体中定义了几个有关 Call 的 crud 方法，以及对 Client 自身状态进行管理的方法
    - 实现了 Client 收发的基本功能。因为我们把一次请求封装成了 Call，我们额外为 Call 添加了一个 Channel，使得对它的收发具有异步的能力
5. 在 service.go 中
    - 因为客户端的请求是 service.method，所以我们分别定义了 service 结构体和 methodType 结构体分别用来保存“一个用来提供服务的结构体”和“其方法被调用所需要“的各项信息
      service 对象代表着一个提供服务的对象，methodType 对象代表着这个提供服务的对象的一个方法。所以 service 以 map 持有多个 methodType。
      提供了将一个结构体对象转变成 service对象，其中的合规方法转变成 methodType 的函数
    - 将 service 集成进 Server，使得 Server能够注册 service，持有多个 service，并且在处理请求时，能够调用到对应 service 的 method 上
6. 在 client.go、server.go 中
    - 为 client.go 中 Dial 函数添加了连接的超时处理的逻辑，连接的超时包括 net.Dial 和 构造客户端 两部分。（使用 time.After() 结合 select+chan 完成）
      为 Call 方法添加了调用超时的逻辑。（使用 context 来完成，context 的好处是：不仅可以完成超时的功能，还有 WithValue、WithCancel 等功能）
    - 为 server.go 中的 handleRequest 方法添加了超时处理逻辑，主要是反射调用目标方法的超时时。使用 time.After() 结合 select+chan 完成）
    - 添加了对超时逻辑的测试代码
7. 在 server.go、client.go 中
    - 在 server.go 中对 Server 实现了 http.Handler 接口，使得 Server 能够作为 Handler 处理 Http 请求。
      支持 Http 协议的整个逻辑是这样的：
          1. 我们想要在开启了 rpc 服务之后，同时也可以通过浏览器观察服务器的一些状态
          2. 需要通过浏览器观察的话，那么就需要启动 Http 服务，而 rpc 服务和 Http 服务都是阻塞服务，无法在同一个协程里面开启。
             而在两个协程分别开启的话，通信又比较麻烦，所以可以进行整合。
          3. 那么，整合的思路是：让 Server 实现 http.Handler，然后启动 Http 服务。
             在处理客户端第一个 Http 请求的时候，服务端通过这个 http conn 切换到 rpc 协议，客户端通过这个 http conn 创建一个 rpc 客户端。
             而这个 http conn 需要通过客户端、服务端通过 Http 协议的 CONNECT 报文交换才能拿到。（不然的话，这个 conn 会在这次请求结束后就被 close）
          4. 既然 Http 服务和 rpc 服务都启动了，那么我们完全可以在 Http 服务启动之前，添加一些别 [路径 -> Handler]，用来处理一些定制化的需求。
             这样用户就可以在浏览器通过这些路径，达到一些定制化的需求了，同时也不会影响我们的 rpc 功能。例如 debug.go
    - 在 server.go 中，实现了 HandleHTTP 方法，用来添加 [路径 -> Handler] 的映射
    - 在 client.go 中，实现了 DialHTTP 方法，使得 Client 能够与服务端进行 CONNECT 连接，并使用该连接的 http conn 切换到 rpc 协议，创建 Client
    - 在 client.go 中，整合了 DialHTTP 和 Dial 方法。
8. 在 discover.go、xclient.go 中
	- 在 discover.go 中抽象出了一个服务发现的接口，所有服务发现的实体应该实现这个接口。同时，实现了一个简单的维护多服务的服务发现接口（其实就是用 []string 保存了服务器的地址）
	- 在 xclient.go 中主要是实现了 XClient 对象。这个对象主要持有一个 Discovery 实例、一个包含 [已注册的服务器的地址 -> 与对应服务器连接的 Client] 的 map
      那么，XClient 的进行一次远程过程调用是这样的：
      	  XClient 使用 Discovery 通过“负载均衡策略”获取一个服务器的 addr，然后通过这个 addr 从上述 map 中取出一个 Client，然后利用这个 Client 发送对应的请求
	- 在 xclient.go 中给 XClient 实现了一个 Broadcast 方法，可以将一个请求广播到所有服务器上，进行处理。
*/


// 多服务器的负载均衡首先得要有服务发现，我们先来实现一个简单的服务发现模块

// Discovery 定义服务发现的通用接口
type Discovery interface {
	Refresh() error                      // 从远程结点更新服务
	Update(servers []string) error       // 手动更新服务
	Get(mode SelectMode) (string, error) // 根据负载均衡策略选择服务
	GetAll() ([]string, error)           // 返回所有服务
}

// SelectMode Discovery 的 Get 方法需要根据负载均衡策略来选择服务
type SelectMode int

// 定义几种常见的负载均衡策略常量
const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
	// ConsistentHash TODO
	ConsistentHash
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
		mu:      sync.RWMutex{}, // 由于 Go 存在默认值，所以也可以不用显示的构造
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

func (m *MultiServersDiscovery) Refresh() error {
	// 因为是手工维护的，远程刷新并不需要，所以不做处理
	return nil
}

func (m *MultiServersDiscovery) Update(servers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func (m *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	// TODO 思考 Get 和 GetAll 使用的锁为什么不一样？这个里面会修改 m.index，所以需要上写锁
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return m.servers[m.r.Intn(n)], nil
	case RoundRobinSelect:
		s := m.servers[m.index%n]
		m.index = (m.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: no supported select mode")
	}
}

func (m *MultiServersDiscovery) GetAll() ([]string, error) {
	m.mu.RLock()
	m.mu.RUnlock()
	srvs := make([]string, len(m.servers), len(m.servers))
	// 返回拷贝
	copy(srvs, m.servers)
	return srvs, nil
}

var _ Discovery = (*MultiServersDiscovery)(nil)
