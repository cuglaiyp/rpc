package xclient

import (
	"context"
	"geerpc/config"
	"github.com/go-zookeeper/zk"
	"log"
	"time"
)

// 前七天我们实现了服务端、客户端、服务注册、超时处理、支持 Http 协议、还有多服务器负载均衡，以及注册中心
// 今天我们将注册中心扩展至 zookeeper 和 etcd

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
10. 在 consistenthash.go、discovery_zk.go、discovery_etcd、zookeeper.go、etcd.go、config.go 中
	- 在 consistenthash.go 中封装了一个一致性 hash 类。
	  原理就是：针对给的真实结点名称，加上虚拟结点编号，得出这些虚拟结点的 hash，然后做个映射 [虚拟结点 hash -> 真实结点名称]。
      然后根据某个服务，查找一个最靠近的虚拟结点，并取出其对应的真实结点即可。由于是 hash 环，并且要利于查找，所以虚拟结点 hash 不光做映射，还需要有序存入切片中
      这里实现得不是很好：每次 Get 的时候，都会重新计算并生成 hash 环。还可以改进。
	- 在 discovery_zk.go、discovery_etcd 分别实现了针对两个平台的服务发现功能，并利用这两个平台的 watch 机制，实现了 key 的监听，一旦服务器上下线，就能及时更新
      zookeeper 的通知只是一个通知，并且只能单次触发；而 etcd 的通知不仅可以多次触发，而且会告知具体的事件，比如：添加、删除，所以在 etcd 的服务发现可以做得更精细。
	- 在 zookeeper.go、etcd.go 实现了服务注册以及心跳保持机制的客户端，服务启动时，可以使用这两个文件下的客户端，进行服务注册和心跳保持。
	- 在 config.go 抽取了服务注册和服务发现共同需要的一些常量。
*/

type ZkRegistryDiscovery struct {
	*GeeRegistryDiscovery               // 继承这个，可以复用 Update 方法
	conn                  *zk.Conn      // 我们自己的注册中心是 Http 服务，而 Zookeeper 就维护一个连接
	timeout               time.Duration // 服务列表的过期时间
	lastUpdate            time.Time     // 上一次更新时间从注册中心拉取服务的时间
	cancelWatch           func()        // 关闭 watch
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
	ctx, cancelFunc := context.WithCancel(context.Background())
	g.cancelWatch = cancelFunc
	go g.watchProviders(ctx)
	return g
}

func (g *ZkRegistryDiscovery) watchProviders(ctx context.Context) {
	for {
		// 循环监听提供服务结点的子结点是否发生变化（因为这个监听是一次性的，使用之后就会失效，所以需要循环注册）
		_, _, event, err := g.conn.ChildrenW(config.ZkProviderPath)
		if err != nil {
			return
		}
		select {
		case <-event:
			g.refreshFromZk()
		case <-ctx.Done():
			break
		}
		// 子结点产生了变化，就从服务器拉取
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
	g.cancelWatch()
	g.conn.Close()
	return nil
}

var _ Discovery = (*ZkRegistryDiscovery)(nil)
