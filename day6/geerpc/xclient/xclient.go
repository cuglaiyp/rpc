package xclient

import (
	"context"
	. "geerpc"
	"io"
	"reflect"
	"sync"
)

// 简易的服务发现模块写完后，写一个带有负载均衡的客户端

type XClient struct {
	mode    SelectMode         // 负载均衡那么需要负载均衡策略
	d       Discovery          // 需要有服务发现的模块
	mu      sync.Mutex         // 需要一个锁
	clients map[string]*Client // 一个通用客户端的集合，主要是为了资源复用。[rpcAddr -> *Client]
	opt     *Option            // 既然 XClient 是面向用户的接口，那么也得给给用户可定制的操作
}

// 因为是客户端，所以需要实现 io.Closer 接口
var _ io.Closer = (*XClient)(nil)

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	// 挨个关闭并移除
	for nm, client := range xc.clients {
		_ = client.Close()
		delete(xc.clients, nm)
	}
	return nil
}

// NewXClient 接着来一个构造方法
func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
	return &XClient{
		mode:    mode,
		d:       d,
		mu:      sync.Mutex{},
		clients: map[string]*Client{},
		opt:     opt,
	}
}

// 既然是客户端，那必然要有 dial方法连接服务端

func (xc *XClient) dial(rpcAddr string) (*Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client, ok := xc.clients[rpcAddr]
	if ok && !client.IsAvailable() { // 这个客户端存在但是不可用，那么移除它
		_ = client.Close()
		delete(xc.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		var err error
		client, err = XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

// 方法签名与 Client 类似，因为底层就是用的 Client
func (xc *XClient) call(ctx context.Context, rpcAddr string, serviceMethod string, args, reply interface{}) error {
	// 通过服务器地址，拿到与该 Server 对应的 Client
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// Call 为 call 方法封装上负载均衡策略，并对外暴露
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(ctx, rpcAddr, serviceMethod, args, reply)
}

// 至此，一个带服务发现的负载均衡客户端已经完成。
// 接着我们来给这个客户端添加一个额外的广播功能：将一个请求广播到所有服务器上

func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 既然要广播到所有服务器上，那么就需要拿到所有服务器
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}
	/*
		// 向所有服务器请求
		for _, rpcAddr := range servers {
			// 并发请求
			go xc.call(ctx, rpcAddr, serviceMethod, args, reply)
		}
		return nil
	*/
	/*
		对上面的代码加一些并发相关的细节：
			1. 并发请求所有服务。需要使用互斥锁。
			2. 需要等所有服务都返回才能继续。需要使用 WaitGroup。
			3. 有任一错误发生时，快速失败。需要使用 context。
			4. 调用成功，则返回其中一个结果。
	*/
	var wg sync.WaitGroup
	var mu sync.Mutex
	var er error
	replyDone := reply == nil // reply 为 nil 或者 reply 已经被赋过一次值，都不需要再赋值
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var cloneReply interface{}
			if reply != nil {
				//                                Value     具体的Value  具体Value的Type
				//                                 |              |      |
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
				//                    |                                            |
				//          根据具体Value的Type new一个新Value                   返回新Value内部的值
			}
			err := xc.call(ctx, rpcAddr, serviceMethod, args, cloneReply)
			mu.Lock()
			if err != nil && er == nil { // 判断是否有失败
				er = err
				cancel() // 调用父 ctx 的 cancel 方法，取消所有还在等待调用的协程
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return er
}
