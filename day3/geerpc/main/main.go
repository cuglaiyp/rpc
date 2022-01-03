package main

import (
	"geerpc"
	"log"
	"net"
	"sync"
	"time"
)



func main() {
	log.SetFlags(0)
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
	geerpc.Accept(listener)
}

func startClient(addr chan string) {
	client, _ := geerpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }()
	time.Sleep(time.Second)
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
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Bar error: ", err)
			}
			log.Println("reply:", reply)
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
