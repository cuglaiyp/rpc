package geerpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 在第一天，完成了服务端的收发请求，今天来完成客户端

// 1. 在 codec.go 中
//    - 我们抽象了编解码器的接口 Codec，设计好了 Header（Body 目前还不知道存放什么东西，先放着）
//    - 定义了编解码器的种类，以及存放编解码器构造函数的容器
// 2. 在 gob.go 中
//    - 我们定义了一个具体的编解码器：GobCodec，实现了 Codec
//    - 实现了该编解码器的构造函数
// 3. 在 server.go 中
//    - 我们约定好了 rpc 报文的标识字段，封装成了 Option，并制定用 Json 解析
//    - 我们定义好了 server，并完成了初步功能
//      |- 1. 解析 Option
//		|- 2. 判断魔数，根据编解码协议，解码内容
//		|- 3. 根据内容处理请求，并响应
//      在请求处理的过程中，因为一个请求报文中可以装多个 Header 和 Body 对，所以使用了并发处理，并且因为这些报文对在同一个连接中，
//      那么需要在所有 goroutine 结束后，才能关闭连接，所以使用了 waitGroup；又因为 Header 和 Body 是成对的，所以在局部写入的
//      时候，使用了 mutex 来保证它们的原子性。
//    - 添加了默认 server 并且，添加了包名启动服务的函数
//    - 注意 154: var h *codec.Header 的坑，这里的 h 是 nil，所以解析会报错。推荐用 h := &codec.Header{}
// 4. 在 client.go 中
//    - 我们将一次调用封装成一个 Call，并且在发送请求的时候，将这个 Call 的字段设置分成 Header 和 Body 写到服务端。
//      所以 Client 需要持有 Call 的集合和一个可复用的 Header
//    - 因为涉及到 Call 的操作，所以我们在 Client 结构体中定义了几个有关 Call 的 crud 方法，以及对 Client 自身状态进行管理的方法
//    - 实现了 Client 收发的基本功能。因为我们把一次请求封装成了 Call，我们额外为 Call 添加了一个 Channel，使得对它的收发具有异步的能力

// 在第三天，完成了服务的注册，至此客户端服务端整体功能基本完成，今天来完成超时控制

// 1. 在 codec.go 中
//    - 我们抽象了编解码器的接口 Codec，设计好了 Header（Body 目前还不知道存放什么东西，先放着）
//    - 定义了编解码器的种类，以及存放编解码器构造函数的容器
// 2. 在 gob.go 中
//    - 我们定义了一个具体的编解码器：GobCodec，实现了 Codec
//    - 实现了该编解码器的构造函数
// 3. 在 server.go 中
//    - 我们约定好了 rpc 报文的标识字段，封装成了 Option，并制定用 Json 解析
//    - 我们定义好了 server，并完成了初步功能
//      |- 1. 解析 Option
//		|- 2. 判断魔数，根据编解码协议，解码内容
//		|- 3. 根据内容处理请求，并响应
//      在请求处理的过程中，因为一个请求报文中可以装多个 Header 和 Body 对，所以使用了并发处理，并且因为这些报文对在同一个连接中，
//      那么需要在所有 goroutine 结束后，才能关闭连接，所以使用了 waitGroup；又因为 Header 和 Body 是成对的，所以在局部写入的
//      时候，使用了 mutex 来保证它们的原子性。
//    - 添加了默认 server 并且，添加了包名启动服务的函数
//    - 注意 154: var h *codec.Header 的坑，这里的 h 是 nil，所以解析会报错。推荐用 h := &codec.Header{}
// 4. 在 client.go 中
//    - 我们将一次调用封装成一个 Call，并且在发送请求的时候，将这个 Call 的字段设置分成 Header 和 Body 写到服务端。
//      所以 Client 需要持有 Call 的集合和一个可复用的 Header
//    - 因为涉及到 Call 的操作，所以我们在 Client 结构体中定义了几个有关 Call 的 crud 方法，以及对 Client 自身状态进行管理的方法
//    - 实现了 Client 收发的基本功能。因为我们把一次请求封装成了 Call，我们额外为 Call 添加了一个 Channel，使得对它的收发具有异步的能力
// 5. 在 service.go 中
//    - 因为客户端的请求是 service.method，所以我们分别定义了 service 结构体和 methodType 结构体分别用来保存“一个用来提供服务的结构体”和“其方法被调用所需要“的各项信息
//      service 对象代表着一个提供服务的对象，methodType 对象代表着这个提供服务的对象的一个方法。所以 service 以 map 持有多个 methodType。
//      提供了将一个结构体对象转变成 service对象，其中的合规方法转变成 methodType 的函数
//    - 将 service 集成进 Server，使得 Server能够注册 service，持有多个 service，并且在处理请求时，能够调用到对应 service 的 method 上
// 6. 在 client.go、server.go 中
//    - 为 client.go 中 Dial 函数添加了连接的超时处理的逻辑，连接的超时包括 net.Dial 和 构造客户端 两部分。（使用 time.After() 结合 select+chan 完成）
//      为 Call 方法添加了调用超时的逻辑。（使用 context 来完成，context 的好处是：不仅可以完成超时的功能，还有 WithValue、WithCancel 等功能）
//    - 为 server.go 中的 handleRequest 方法添加了超时处理逻辑，主要是反射调用目标方法的超时时。使用 time.After() 结合 select+chan 完成）
//    - 添加了对超时逻辑的测试代码
//

// Client 需要哪些字段？
// - Codec 需要，因为需要编码消息
// - Header 需要，这是我们的消息头
// - Option 需要，需要与服务端协商报文格式
// - 可能外加的一些 bool，表示客户端的状态，以及一些锁，用来保持同步
type Client struct {
	cc     codec.Codec
	opt    *Option
	header codec.Header

	seq     uint64
	pending map[uint64]*Call // [seq: *Call]

	closing  bool // 用来表示用户主动关闭的字段
	shutdown bool // 用来表示服务端告诉我们关闭的字段

	sending sync.Mutex // 用来保持发送同步的锁
	mu      sync.Mutex // 用在各种需要同步的地方
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	return c.cc.Close()
}

var _ io.Closer = (*Client)(nil)

func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closing && !c.shutdown
}

// 客户端的行为是什么样？
// - 首先新建客户端的时候，它就应该与服务端进行通信，发送Option，交换报文格式
// - 客户端需要有发送报文的能力，发送报文发送的是什么？是一次调用，所以这个调用我们也可以封装成一个类，由客户端持有
// - 接收报文的能力，这个就正常处理

// Call 表示一次远程方法的调用
type Call struct {
	Seq           uint64      // 调用序号
	ServiceMethod string      // 服务.方法
	Args          interface{} // 参数
	Reply         interface{} // 返回结果
	Error         error       // 调用过程中出现的错误
	Done          chan *Call  // 用来实现异步请求的工具
}

func (c *Call) done() {
	c.Done <- c
}

// 既然客户端需要持有Call，那么就得有注册、移除等方法

// registerCall 注册一个 Call ，返回这个 call 的在客户端中的键和 error
func (c *Client) registerCall(call *Call) (uint64, error) {
	// 涉及并发，加锁
	c.mu.Lock()
	defer c.mu.Unlock()
	// 注册之前，判断一下客户端的状态
	if c.shutdown || c.closing {
		log.Printf("rpc client: registerCall: %s", ErrShutdown.Error())
		return 0, ErrShutdown
	}
	// 选一个 seq，那么我们可以在 Client 里面添加一个 seq 的计数器，只是向上增长
	call.Seq = c.seq
	c.pending[c.seq] = call
	c.seq++ // 这个是语句，并不产生结果；注意与表达式进行区分
	return call.Seq, nil
}

var ErrShutdown = errors.New("connection is shut shutdown")

// removeCall 用键移除
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

func (c *Client) terminateCalls(err error) {
	// TODO 思考为什么要同时持有两把锁
	c.sending.Lock() // 先上 sending 锁，与发送操作形成互斥
	defer c.sending.Unlock()
	c.mu.Lock() // 上完 sending 锁之后，再上 mu 锁
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		// 设置了 err 之后，因为是异步的，所以还要通知一下等待这个 call 完成的协程
		// call.Done <- call 把这个封装成方法
		call.done()
	}
}

// 封装好了客户端（Client） 和 调用（Call），现在可以来实现客户端的功能了。主要有两个，发送请求和接收响应

// send 直接针对 call 发送一次请求
func (c *Client) send(call *Call) {
	// 1. 得上锁，因为装 Call 的 map 存在争用
	c.sending.Lock()
	defer c.sending.Unlock()

	// 2. 将这个 call 注册进 map 里面，发送完请求后，等待接收
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 3. 发送本次请求
	// 3.1 构建消息头
	c.header.Seq = seq
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Error = ""
	// 3.2 编码并发送
	if err := c.cc.Write(&c.header, call.Args); err != nil {
		call := c.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
		return
	}
}

func (c *Client) receive() {
	// 死循环接收请求
	var err error
	for err == nil {
		// 1. 先读头
		header := &codec.Header{}
		if err := c.cc.ReadHeader(header); err != nil {
			break
		}
		// 2. 再读体
		// 拿到头之后，知道了seq，先调用 removeCall 方法移除并得到这个 call
		call := c.removeCall(header.Seq)
		switch {
		case call == nil:
			// 意味着这个 call 往服务端发送失败了，它已经被移除了
			err = c.cc.ReadBody(nil)
		case header.Error != "":
			// 服务端解析、调用过程中出现错误
			call.Error = fmt.Errorf(header.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			// 正常情况
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = fmt.Errorf("reading body: %s", err.Error())
			}
			call.done()
		}
	}
	// 客户端、服务端发送错误时，终结所有 call，并通知错误消息
	c.terminateCalls(err)
}

// 到此为止，客户端的功能基本完成了。下面再给客户端加上一些方便使用的函数和方法

// NewClient
// 1. 通过上面的代码可知，客户端的收发请求都是通过 codec 来完成了，codec 需要“连接”，所以这里的参数之一是“连接”
// 2. 简易性的原则使得我们想要 new 了一个客户端之后，立马可以收发消息，而不需要事后还要与服务端协商编码。那么这里的参数之二就是用于协商编码的“Option”
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	codecFunc := codec.NewCodecFuncMap[opt.CodecType]
	if codecFunc == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error: ", err)
		return nil, err
	}
	// 协商编码
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: option err: ", err)
		_ = conn.Close()
		return nil, err
	}
	// 接收到服务端的确认消息，完成两次握手。如果没有这第二次握手，
	// 会产生 rpc server: read header error: gob: unknown type id or corrupted data 的粘包问题
	// 		- 当客户端消息发送过快服务端消息积压时（例：Option|Header|Body|Header|Body），
	//		服务端使用json解析Option，json.Decode()调用conn.read()读取数据到内部的缓冲区（例：Option|Header），
	//		此时后续的RPC消息就不完整了(Body|Header|Body)。 示例代码中客户端简单的使用time.sleep()方式隔离协议交换阶段与RPC消息阶段，减少这种问题发生的可能。
	if err := json.NewDecoder(conn).Decode(opt); err != nil {
		log.Println("rpc client: option err: ", err)
		_ = conn.Close()
		return nil, err
	}
	cc := codecFunc(conn)
	c := &Client{
		cc:       cc,
		opt:      opt,
		header:   codec.Header{},
		seq:      1, // 从 1 开始，0 表示非法调用
		pending:  make(map[uint64]*Call),
		closing:  false,
		shutdown: false,
	}
	// 更近一步，启动接收协程
	go c.receive()
	return c, nil
}

// conn 怎么来？ net.Dial(network, address)！ opt 怎么来？自己构造！

// Dial 为了优化上面这两个操作，我们再包装一层
func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	/*if len(opts) > 1 {
		return nil, errors.New("option 数量过多")
	}
	opt := DefaultOption
	if len(opts) != 0 && opts[0] != nil {
		opt = opts[0]
	}
	opt.MagicNumber = MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = codec.GobType
	}*/

	/*opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()
	return NewClient(conn, opt)*/

	// 有了 dialTimeout，修改一下 Dial 的逻辑
	return dialTimeout(NewClient, network, address, opts...)
}

// 为 Dial 操作增加连接超时的逻辑
// 这里的 ConnectTimeout 有两层意思: 1. net.Dial 的时候要短于 ConnectTimeout; 2. 同时创建 Client 整体处理时间也需要短于 ConnectTimeout
func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	// 将 dialTimeout 和 Dial 中的这段代码抽取
	/*if len(opts) > 1 {
		return nil, errors.New("option 数量过多")
	}
	opt := DefaultOption
	if len(opts) != 0 && opts[0] != nil {
		opt = opts[0]
	}
	opt.MagicNumber = MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = codec.GobType
	}*/
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	// 连接超时直接返回
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	// 连接没有超时的话，继续判断处理是否会超时。
	// 这里需要用到 Channel。因为 Channel 里面只能装一个结构体，所以我们将两个返回值，封装成一个结构体 clientResult
	ch := make(chan clientResult)
	// 开一个子协程去创建客户端
	go func() {
		cs := clientResult{}
		// 为了更定制化一些，将能够生成 Client 的函数抽象成一个类型，由参数传进来
		// cs.client, cs.err = NewClient(conn, opt)
		cs.client, cs.err = f(conn, opt)
		// ch <- cs
		// 这里会有内存泄露的隐患：超时之后，由于没有 <-ch，这个子协程会阻塞在 ch <- cs
		// 有两种解决方案：
		//    1. 把 ch 改成缓冲形式的 channel：ch := make(chan clientResult, 1)
		//    2. 使用 select + default，结果不能放入 ch 的话，就走 default
		// 样例代码 ：
		// // ch := make(chan struct{}, 1)
		//	ch := make(chan struct{})
		//	timeout := time.Second * 2
		//	go func() {
		//		time.Sleep(time.Second * 4)
		//		/*select {
		//		case ch <- struct{}{}:
		//		default:
		//		}*/
		//		ch <- struct{}{}
		//		fmt.Println("协程正常退出")
		//	}()
		//	select {
		//	case <-time.After(timeout):
		//		fmt.Println("超时")
		//	case <-ch:
		//		fmt.Println("正常退出")
		//	}
		//	for{
		//
		//	}
		select {
		case ch <- cs:
		default:
		}
	}()

	// 如果超时时间设置的是 0 话，说明不需要超时处理，等到连接完成再返回
	if opt.ConnectTimeout == 0 {
		res := <-ch
		return res.client, res.err
	}

	// 判断创建客户端的处理是否超时
	select {
	case <-time.After(opt.ConnectTimeout): // 超时时间到了，从这里返回
		return nil, fmt.Errorf("rpc client: connect timeout with %s", opt.ConnectTimeout)
	case res := <-ch: // 超时时间内处理完成，从这里返回
		return res.client, res.err
	}
}

func parseOptions(opts ...*Option) (*Option, error) {
	if len(opts) > 1 {
		return nil, errors.New("option 数量过多")
	}
	opt := DefaultOption
	if len(opts) != 0 && opts[0] != nil {
		opt = opts[0]
	}
	opt.MagicNumber = MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = codec.GobType
	}
	return opt, nil
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *Option) (*Client, error)

// 通过 Dial 已经能够很方便地连接服务器了。除此之外，我们再来把 send 方法包装一下，因为它的参数有 Call，需要自己构造，不方便

// Go 异步调用。我们发一起一次调用，只需要指定我们的：service.methods、入参、结果。同时，为了支持异步，添加了一个 chan 参数
func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

// Call 同步调用。只需要在 Go 上面做同步上就好了
// 给 Call 加上请求超时机制，利用 Context 来做
func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done(): // 调用超时
		return fmt.Errorf("rpc client: call failed: " + ctx.Err().Error())
	case ca := <-call.Done: // 调用完成
		return ca.Error
	}
}

// ---
// 在前面 4 天，完成了服务端、客户端、服务注册、超时处理等功能的编写。现在完成客户端支持 Http 协议的功能

// 目前，服务端已经能够处理 Http 请求，并且将 Http 请求转换成我们的 rpc

func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	// 给服务端写一句话。使用 CONNECT 方法的报文格式
	// 写入 http 请求消息，格式为：请求行\n请求头（头部行）\n请求体。由于没有加请求头（头部行），所以这里末尾写了两个\n
	io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))
	// 要求服务端以 CONNECT 方法返回
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err == nil && resp.Status == connected {
		// Http 连接建立之后，再继续建立我们的 rpc 连接，返回我们的 rpc 客户端
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

// DialHTTP 同样为了方便，给 NewHTTPClient 函数也包装一层
func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

// 最后，由于有两个 Dial（Dial、DialHTTP），把这两个方法再包装一层。

// XDial 去掉 network 参数，通过指定的 address 格式区分网络。例：http@127.0.0.1:8080)
func XDial(address string, opts ...*Option) (*Client, error) {
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client: wrong format '%s', expect protocol@addr", address)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		// http 协议底层本身还是基于 tcp 的，所以这里网络要填入 tcp。
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}
