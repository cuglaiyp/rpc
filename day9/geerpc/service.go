package geerpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// 在前一天中，我们把客户端弄好了，这个时候服务端和客户端可以进行通信了。但是服务端对客户端请求的处理仅仅是打印一下，而没有真正调用一个方法。
// 今天来处理这个问题

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
//      那么出了错误之后，需要在所有 goroutine 结束后，才能关闭连接，所以使用了 waitGroup；又因为 Header 和 Body 是成对的，所以在局部写入的
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

// 结构体就是一个服务，结构体的方法就是服务的方法。我们首先定于服务（service）
type service struct {
	name    string                 // 服务的名字
	typ     reflect.Type           // 服务的类型
	rcvr    reflect.Value          // 服务的实体
	methods map[string]*methodType // 服务的方法（name: methods)
}

// 因为服务里面需要方法，我们继续把方法也定义出来
type methodType struct {
	method    reflect.Method // 方法实体
	ArgType   reflect.Type   // 第一个参数（入参）类型
	ReplyType reflect.Type   // 第二个参数（返回值）类型
	numCalls  uint64         // 统计这个方法调用的次数
}

// 因为 methodType 里面 ArgType、ReplyType 都是 reflect.Type 类型，所以我们给 methodType 添加两个方法，让这两个字段能变成类型的值

func (m *methodType) NewArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem()) // 返回指针指向的具体值
	} else {
		argv = reflect.New(m.ArgType).Elem() // 返回接口承载的具体值
	}
	return argv
}

func (m *methodType) NewReplyv() reflect.Value {
	// 返回值必须要是指针类型 ptr
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() { // 判断 Reply 指向或装载的具体类型
	case reflect.Map:
		// ptr -> (replyv) map
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		// ptr -> (replyv) slice
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv // 或者 ptr -> (replyv) otherType
}

// NumCalls 再给 numCalls 加一个 Get 方法
func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

// 接着给 service 添加构造方法，将结构体映射为服务
func newService(rcvr interface{}) *service {
	s := &service{}
	s.rcvr = reflect.ValueOf(rcvr)
	s.typ = reflect.TypeOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

func (s *service) registerMethods() {
	// func (s *service) methodName(argv, replyv) error {}
	s.methods = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mt := method.Type
		if mt.NumIn() != 3 || mt.NumOut() != 1 { // 入参加接收器 3 个，返回值 1 个
			continue
		}
		if mt.Out(0) != reflect.TypeOf((*error)(nil)).Elem() { // 返回值类型
			continue
		}
		argType, replyType := mt.In(1), mt.In(2)
		if !isExportedOrBuiltin(argType) || !isExportedOrBuiltin(replyType) {
			continue
		}
		s.methods[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
			numCalls:  0,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltin(typ reflect.Type) bool {
	return ast.IsExported(typ.Name()) || typ.PkgPath() == ""
}

// 接着写一个 service 调用注册进服务的方法

func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	rtval := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := rtval[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

// 目前 service 已经写完，要将这个服务嵌入进 server 中，给它调用
// 1. server 需要能够注册 service，那么 server 需要能够注册，并且需要持有 service
// 2. 处理服务时，根据 service.methods 调用 service 中的 methodType
