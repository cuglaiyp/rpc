package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// 前六天我们实现了服务端、客户端、服务注册、超时处理、支持 Http 协议、还有多服务器负载均衡
// 今天我们完成最后一块：注册中心

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
9. 在 registry.go、discovery_gee.go 中
	- 在 registry.go 中，实现了一个简易的注册中心，这个注册中心维护了一个 [服务器地址 -> 服务器地址 + 该服务上一次的心跳时间] 的 map，
      并且通过实现 http.Handler 接口，对外提供 Http 服务，这样每个服务器可以通过 POST 请求发送心跳、服务发现模块通过 GET 请求拉取所有可用服务器的地址
	- 在 registry.go 中，还暴露了对外的 Heartbeat 函数，使得服务可以使用该函数向指定注册中心发送指定服务的心跳
	- 在 discovery_gee.go 中，实现了一个 GeeDiscovery 服务发现模块，继承了 MultiServersDiscovery 类，并重写了 Discovery 接口，
      使得该模块可以使用 GET 请求从其维护的注册中心远程拉取可用服务
*/

// 定义一个注册中心接口

type Registry interface {

}


// 首先定义一个注册中心的实例

type GeeRegistry struct {
	timeout time.Duration          // 超时时间，服务注册后存活超过这个时间就死亡
	mu      sync.Mutex             // 锁
	servers map[string]*ServerItem // 由于存在超时时间等属性，服务器不能只用一个地址代替了，所以我们添加一个新的结构体
}

type ServerItem struct {
	Addr  string    // 对应服务器的地址
	start time.Time // 该服务上一次心跳的时间
}

// New 构造方法，timeout 为超时时间
func New(timeout time.Duration) *GeeRegistry {
	return &GeeRegistry{
		timeout: timeout,
		mu:      sync.Mutex{},
		servers: map[string]*ServerItem{},
	}
}

// 接着定义几个常量

const (
	defaultPath    = "/_geerpc_/registry"
	defaultTimeout = time.Minute * 5 // 默认的超时时间为 5 分
)

// 来一个默认的注册中心

var DefaultRegistry = New(defaultTimeout)

// 注册中心功能有：添加服务实例、返回可用服务列表。给这两个功能加上

func (g *GeeRegistry) putServer(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.servers[addr]
	if s == nil { // 不存在就添加
		g.servers[addr] = &ServerItem{
			Addr:  addr,
			start: time.Now(),
		}
	} else { // 存在就更新时间
		s.start = time.Now()
	}
}

func (g *GeeRegistry) aliveServers() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var servers []string
	for addr, si := range g.servers {
		if g.timeout == 0 || si.start.Add(g.timeout).After(time.Now()) { // 注册中心没有超时时间或者服务还没失活
			servers = append(servers, addr)
		} else { // 删除已经不可用的服务
			delete(g.servers, addr)
		}
	}
	// 保证每次调用的相对顺序不变（幂等？）
	sort.Strings(servers)
	return servers
}

// 注册中心基础功能都完成了，接下来让它能够对外提供服务。这里为了实现上的简单，采用 Http 协议，且所有的有用信息都承载在 HTTP Header 中

// 实现 http.Handler 接口
var _ http.Handler = (*GeeRegistry)(nil)

// Get：返回所有可用的服务列表，通过自定义字段 X-Geerpc-Servers 承载。
// Post：添加服务实例或发送心跳，通过自定义字段 X-Geerpc-Server 承载。
func (g *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		// 将所有可用服务写在响应头上
		w.Header().Set("X-Geerpc-Servers", strings.Join(g.aliveServers(), ","))
	case http.MethodPost:
		// 从请求头上获取服务
		addr := req.Header.Get("X-Geerpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		g.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// 为注册中心添加映射路径

func (g *GeeRegistry) HandleHTTP(registryPath string)  {
	http.Handle(registryPath, g)
	log.Println("rpc registry path:", registryPath)
}

// 实现包名调用简易方法

func HandleHTTP() {
	DefaultRegistry.HandleHTTP(defaultPath)
}


// 实现一个包名调用的辅助函数（helper function），便于服务启动后定时向注册中心发送心跳

// Heartbeat registry 注册中心地址，addr 服务地址
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		//  没给超时时间的话，就以默认的超时间发一次，但是不能卡着点发，因为发送还需要时间
		duration = defaultTimeout - time.Duration(1) * time.Minute
	}
	err := sendHeartbeat(registry, addr)
	go func() {
		ticker := time.NewTicker(duration) // 来一个计时器
		for err == nil { // 没有错误就循环定时发送心跳
			<- ticker.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

// 向注册中心发送心跳

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	// 前面约定过，使用 Http 协议发送心跳
	client := http.Client{}
	request, _ := http.NewRequest(http.MethodPost, registry, nil)
	request.Header.Set("X-Geerpc-Server", addr)
	if _, err := client.Do(request); err != nil {
		log.Println("rpc server: heart beat err: ", err)
		return err
	}
	return nil
}

// 注册中心完成之后，我们的服务发现模块还是个简易版本的，现在给它完成
