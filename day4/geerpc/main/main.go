package main

import (
	"context"
	"geerpc"
	"log"
	"net"
	"sync"
)

func main() {
	addr := make(chan string)
	go startClient(addr)
	startServer(addr)
}

func startServer(addr chan string) {
	if err := geerpc.Register(new(Foo)); err != nil {
		log.Fatal("register error:", err)
	}
	listener, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Println("监听端口失败")
		return
	}
	addr <- listener.Addr().String()
	geerpc.Accept(listener) // 启动 rpc 服务
}

func startClient(addr chan string) {
	client, _ := geerpc.Dial("tcp", <-addr)
	defer client.Close()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := Args{
				Num1: i,
				Num2: 6,
			}
			var reply int
			// ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			// ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Bar error: ", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}
