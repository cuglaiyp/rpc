package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

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
//      那么需要所有 goroutine 结束后，才能关闭连接，所以使用了 waitGroup；又因为 Header 和 Body 是成对的，所以在局部写入的
//      时候，使用了 mutex 来保证它们的原子性。
//    - 添加了默认 server 并且，添加了包名启动服务的函数
//    - 注意 154: var h *codec.Header 的坑，这里的 h 是 nil，所以解析会报错。推荐用 h := &codec.Header{}

// MagicNumber geerpc报文的魔数
const MagicNumber = 0x3bef5c

// 既然有多个编解码器，那么就需要请求中有一定的字段来标识请求内容放是用什么编码的
// 为了提升性能，一般在报文的最开始会规划固定的字节，来协商相关的信息。
// 比如第 1 个字节用来表示序列化方式，第 2 个字节表示压缩方式，第 3-6 字节表示 header 的长度，7-10 字节表示 body 的长度
// 在这里，我们简单将其封装为 Option 类，并使用 json 来编码（就不使用字节来区分了）
// 即，报文的格式如下所示：
// | Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
// | <------      固定 JSON 编码      ------>  | <-------    编码方式由 CodeType 决定    ------->|
// 在一次连接中，Option 固定在报文的最开始，Header 和 Body 可以有多个，即报文可能是这样的：
// | Option | Header1 | Body1 | Header2 | Body2 | ...

// Option 报文标识字段
type Option struct {
	MagicNumber int        // 魔数，标识这个报文的类型
	CodecType   codec.Type // 标识请求消息的编码方式

	// 因为超时时间应该也是由 Client 和 Server 协商来的，所以将超时时间字段放入 Option 中
	ConnectTimeout time.Duration // 连接超时
	HandleTimeout  time.Duration // 处理超时
}

// DefaultOption 客户端要是没传Option，我们就用这个默认的
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.GobType,
	ConnectTimeout: time.Second * 10, // 默认 10s
}

// Server 既然前面加上面，对通信的细节已经敲定了。那么就可以编写服务了
type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

// Accept 服务端通过 Accept 方法，监听连接，来一个处理一个
func (s *Server) Accept(lis net.Listener) {
	for true {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			// 一旦监听失败，直接返回
			return
		}
		// 开一个线程处理连接
		// 处理连接有几步 ：
		// 1. 解析 Option
		// 2. 判断魔数，根据编解码协议，解码内容
		// 3. 根据内容处理请求，并响应
		go s.ServeConn(conn)

	}
}

func (s *Server) ServeConn(conn net.Conn) {
	defer conn.Close()
	// 前面注释已经说了，我们的 Option 默认就先用 json 编解码了
	// TODO Option 编解码换成字节形式
	// 1. 解码 Option
	opt := new(Option)
	err := json.NewDecoder(conn).Decode(opt)
	if err != nil {
		log.Println("rpc server: option error: ", err)
		return
	}
	// 2. 判断魔数
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	// 3. 根据编解码字段解码内容
	newCodecFunc := codec.NewCodecFuncMap[opt.CodecType]
	if newCodecFunc == nil {
		log.Println("rpc server: codec doesn't exist")
		return
	}
	cc := newCodecFunc(conn)
	s.serveCodec(cc, opt.HandleTimeout)
}

var invalidRequest = struct{}{}

func (s *Server) serveCodec(cc codec.Codec, timeout time.Duration) {
	wg := new(sync.WaitGroup)
	sending := new(sync.Mutex)
	// 由前面的注释可知，一次连接中，可能有多个 header、body 对，那么需要循环取出，并进行处理
	for true {
		// 解析出一对 Header Body
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				// req 也为空的话，是读取 Header 出了问题，也就说明这个连接出现了问题。这是没办法恢复的，所以只能 break，关闭连接了
				// log.Println("rpc server: resolve request fail: ", err)
				break
			}
			// req 不为空，Header 没有问题，但是出现了其他错误（service 查找失败、Body 读取失败），那么我么可以往回写入错误信息
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending) // 写回
			continue
		}
		// 每一个请求对，再通过并发进行处理
		// 因为是在一个连接中，所以需要等所有请求都处理完，才能关闭连接。那么我们需要使用 waitGroup
		wg.Add(1)
		// 处理请求需要编解码器，需要加上
		// 因为使用了 waitGroup，所以要加上
		// 虽然是并发处理各个 Header Body 对，但是一对 Header 和 Body 是需要原子操作的，所以要对写回进行同步，那么就要加锁
		go s.handleRequest(cc, req, wg, sending, timeout)
	}
	wg.Wait()
	_ = cc.Close()
}

// 从连接中解析出一对正常的 Header 和 Body
func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	// 1. 编解码器解码 Header
	h, err := readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	// Header 读取没问题的话，就可以准备一个 request 了
	req := &request{h: h}
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.NewArgv()
	req.replyv = req.mtype.NewReplyv()

	// 确保 ReadBody 的 argv 是指针
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	// 2. 编解码器解码 Body
	//// 目前 Body 中的 argv 还没有确认好是什么类型，先用字符串
	//req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(argvi); err != nil {
		// TODO 为什么读取 Body 的时候不需要校验 EOF 呢？
		// 因为读取 Header 时已经进行了 EOF 校验，
		//    - 如果读取 Header 出现了 EOF，那么不会再读 Body 了
		//    - 如果读取 Header 没有出现 EOF，那么按照我们的格式，该 Header 必定会出现与之成对的 Body
		log.Println("rpc server: read argv error: ", err)
		return req, err
	}
	return req, nil
}

func readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	/*
			var a *codec.Header
			var b codec.Header
			c := new(codec.Header)
			d := &codec.Header{}
			fmt.Println(a,    &b,    c,     d)
		               <nil> &{ 0 } &{ 0 } &{ 0 }
			第一种写法是 nil，所以不能用，推荐用 d 这种写法。 妈的，真坑。
	*/
	h := &codec.Header{}
	if err := cc.ReadHeader(h); err != nil {
		// 3. 思考到这里的错误类型得进行一下判断
		// 因为当读到输入流的末尾时，也会出现 EOF 错误。但这是正常的，所以不需要进行日志打印
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read request error: ", err)
		}
		return nil, err
	}
	return h, nil
}

// 读取了 Header 和 Body 后，就需要进行处理了
func (s *Server) handleRequest(cc codec.Codec, req *request, wg *sync.WaitGroup, sending *sync.Mutex, timeout time.Duration) {
	// 处理的过程当然是根据参数去调用方法了
	//// 我们这里先简单处理，直接写回一个返回值即可
	defer wg.Done()
	// log.Print(req.h, "-", req.argv.Elem())

	// 给 call 和 sendResponse 增加超时处理
	called := make(chan struct{})
	sent := make(chan struct{})

	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		// 与客户端一样，这里会有内存泄露风险
		// called <- struct{}{}
		select {
		case called <- struct{}{}:
		default:
			return
		}
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		// req.replyv = reflect.ValueOf(fmt.Sprintf("rpc resp %d", req.h.Seq))
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()
	if timeout == 0 { // 不进行超时处理，调用多长时间就等多长时间
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		// 超时了
		req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
	case <-called:
		<-sent
	}

}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	// 保证 Header 和 Body 写入的原子性
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: send response fail: ", err)
	}
}

type request struct {
	h            *codec.Header // 将 header
	argv, replyv reflect.Value // 和 body（请求参数和返回值） 封装成 request
	// 添加两个字段
	mtype *methodType
	svc   *service
}

// DefaultServer 服务端的处理逻辑完成之后，我们给一个全局默认的服务器，以及一个通过包名就可以启动服务的函数，简化用户使用
var DefaultServer = NewServer()

func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}

// Register 注册服务
func (s *Server) Register(rcvr interface{}) error {
	service := newService(rcvr)
	if _, loaded := s.serviceMap.LoadOrStore(service.name, service); loaded {
		return errors.New("rpc server: service already defined: " + service.name)
	}
	return nil
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// 配套实现查找服务的方法
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service.methods request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)
	mtype = svc.methods[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find methods " + methodName)
	}
	return
}

// ---
// 在前面 4 天，完成了服务端、客户端、服务注册、超时处理等功能的编写。
// 今天使我们的服务端和客户端支持 Http 协议

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
// 7. 在 server.go、client.go 中
//    - 在 server.go 中对 Server 实现了 http.Handler 接口，使得 Server 能够作为 Handler 处理 Http 请求。
//      支持 Http 协议的整个逻辑是这样的：
//          1. 我们想要在开启了 rpc 服务之后，同时也可以通过浏览器观察服务器的一些状态
//          2. 需要通过浏览器观察的话，那么就需要启动 Http 服务，而 rpc 服务和 Http 服务都是阻塞服务，无法在同一个协程里面开启。
//             而在两个协程分别开启的话，通信又比较麻烦，所以可以进行整合。
//          3. 那么，整合的思路是：让 Server 实现 http.Handler，然后启动 Http 服务。
//             在处理客户端第一个 Http 请求的时候，服务端通过这个 http conn 切换到 rpc 协议，客户端通过这个 http conn 创建一个 rpc 客户端。
//             而这个 http conn 需要通过客户端、服务端通过 Http 协议的 CONNECT 报文交换才能拿到。（不然的话，这个 conn 会在这次请求结束后就被 close）
//          4. 既然 Http 服务和 rpc 服务都启动了，那么我们完全可以在 Http 服务启动之前，添加一些别 [路径 -> Handler]，用来处理一些定制化的需求。
//             这样用户就可以在浏览器通过这些路径，达到一些定制化的需求了，同时也不会影响我们的 rpc 功能。例如 debug.go
//    - 在 server.go 中，实现了 HandleHTTP 方法，用来添加 [路径 -> Handler] 的映射
//    - 在 client.go 中，实现了 DialHTTP 方法，使得 Client 能够与服务端进行 CONNECT 连接，并使用该连接的 http conn 切换到 rpc 协议，创建 Client
//    - 在 client.go 中，整合了 DialHTTP 和 Dial 方法。




// 我们知道，服务端要能够处理 Http 连接，得实现 http.Handler 接口
// 然后把服务端这个 Handler 放到 http.Serve 中，这样当 Http 起来后，请求就能够被我们的 ServeHTTP 方法处理
var _ http.Handler = (*Server)(nil)

func (s *Server) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	// 首先判断请求方法类型
	if req.Method != "CONNECT" { // 如果 http 请求的方法不是 CONNECT，则不能处理
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8") // 设置响应头
		writer.WriteHeader(http.StatusMethodNotAllowed)                  // 使用 405 状态码写回响应头
		_, _ = io.WriteString(writer, "405 must CONNECT\n")           // 写回响应体
		// writer.Write([]byte("405 must CONNECT\n")) // 同上
		return
	}
	// 劫持该 http 请求
	conn, _, err := writer.(http.Hijacker).Hijack()
	if err != nil {
		log.Println("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	// io.WriteString(conn, "HTTP/1.0 " + "200 Connected to Gee RPC" + "\n\n") // 抽出一些常量
	// 写回 http 响应消息，格式为：响应行\n响应头（头部行）\n响应体。由于没有加响应头（头部行），所以这里末尾写了两个\n
	_, _ = io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	s.ServeConn(conn)
}

const (
	connected      = "200 Connected to Gee RPC"
	defaultRPCPath = "/_geerpc_"
	defaultDebugPath = "/debug/geerpc"
)

// HandleHttp 为 Server 添加默认映射路径
func (s *Server) HandleHttp() {
	http.Handle(defaultRPCPath, s) // 为 DefaultServeMux 添加[路径：Handler]映射。http.Serve方法如果没有传 Handler 参数，就会使用这个 DefaultServeMux
	http.Handle(defaultDebugPath, debugHTTP{s}) // 为扩展功能添加 http 请求路径。debugHTTP{s}，是的 debugHTTP 中的 ServeHTTP 方法可以拿到 s 中各个服务方法的调用次数
	log.Println("rpc server debug path:", defaultDebugPath)
}

// HandleHttp 为默认 Server 添加以包名调用的简易方法
func HandleHttp() {
	DefaultServer.HandleHttp()
}
