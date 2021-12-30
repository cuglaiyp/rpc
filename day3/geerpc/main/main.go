package main

import (
	"geerpc"
	"log"
	"net"
	"sync"
)

func main() {
	addr := make(chan string)
	go func(addr chan string) {
		var foo Foo
		if err := geerpc.Register(&foo); err != nil {
			log.Fatal("register error:", err)
		}
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			log.Println("监听端口失败")
			return
		}
		addr <- listener.Addr().String()
		geerpc.Accept(listener)
	}(addr)

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
