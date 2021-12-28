package geerpc

import (
	"errors"
	"geerpc/codec"
	"log"
	"sync"
)

// 在第一天，完成了服务端的收发请求，今天来完成客户端

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

// 既然客户端需要持有Call，那么就得有注册、移除等方法

// registerCall 注册一个 Call ，返回这个 call 的在客户端中的键和 error
func (c *Client) registerCall(call *Call) (uint64, error) {
	// 涉及并发，加锁
	c.mu.Lock()
	defer c.mu.Unlock()
	// 注册之前，判断一下客户端的状态
	if c.shutdown || c.closing {
		log.Printf("rpc client: RegisterCall: ", ErrShutdown.Error())
		return 0, ErrShutdown
	}
	// 选一个 seq，那么我们可以在 Client 里面添加一个 seq 的计数器，只是向上增长
	c.seq++ // 这个是语句，并不产生结果；注意与表达式进行区分
	c.pending[c.seq] = call
	return c.seq, nil
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