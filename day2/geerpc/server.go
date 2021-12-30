package geerpc

import (
	"encoding/json"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
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
}

// DefaultOption 客户端要是没传Option，我们就用这个默认的
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// Server 既然前面加上面，对通信的细节已经敲定了。那么就可以编写服务了
type Server struct{}

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
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Println("rpc server: codec doesn't exist")
		return
	}
	cc := f(conn)
	s.serveCodec(cc)
}

var invalidRequest = struct{}{}

func (s *Server) serveCodec(cc codec.Codec) {
	wg := new(sync.WaitGroup)
	sending := new(sync.Mutex)
	// 由前面的注释可知，一次连接中，可能有多个 header、body 对，那么需要循环取出，并进行处理
	for true {
		// 解析出一对 Header Body
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				// req 也为空的话，是没办法恢复的，所以只能 break，关闭连接了
				log.Println("rpc server: resolve request fail: ", err)
				break
			}
			// 目前好像是不存在这种场景的
			// req 不为空，但是出现了错误，那么往回写入错误信息
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
		go s.handleRequest(cc, req, wg, sending)
	}
	wg.Wait()
	_ = cc.Close()
}

// 从连接中解析出一对正常的 Header 和 Body
func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	// 1. 编解码器解码 Header
	/*
			var a *codec.Header
			var b codec.Header
			c := new(codec.Header)
			d := &codec.Header{}
			fmt.Println(a, &b, c, d)
		               <nil> &{ 0 } &{ 0 } &{ 0 }
			第一种写法是 nil，所以不能用，推荐用 d 这种写法。 妈的，真坑。
	*/
	h := &codec.Header{}
	if err := cc.ReadHeader(h); err != nil {
		// 3. 思考到这里的错误类型得进行一下判断
		// 因为当读到输入流的末尾时，也会出现 EOF 错误，但这是正常的
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read request error: ", err)
			return nil, err
		}
	}
	// 准备一个 request
	req := &request{h: h}
	// 2. 编解码器解码 Body
	// 目前 Body 中的还没有确认好 argv 还没有确认好是什么类型，先用字符串
	req.argv = reflect.New(reflect.TypeOf(""))
	if err := cc.ReadBody(req.argv.Interface()); err != nil {
		// TODO 为什么读取 Body 的时候不需要校验 EOF 呢？
		log.Println("rpc server: read argv error: ", err)
		return nil, err
	}
	return req, nil
}

// 读取了 Header 和 Body 后，就需要进行处理了
func (s *Server) handleRequest(cc codec.Codec, req *request, wg *sync.WaitGroup, sending *sync.Mutex) {
	// 处理的过程当然是根据参数去调用方法了
	// 我们这里先简单处理，直接写回一个返回值即可
	defer wg.Done()
	log.Print(req.h, "-", req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("rpc resp %d", req.h.Seq))
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
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
}

// DefaultServer 服务端的处理逻辑完成之后，我们给一个全局默认的服务器，以及一个通过包名就可以启动服务的函数，简化用户使用
var DefaultServer = NewServer()

func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}
