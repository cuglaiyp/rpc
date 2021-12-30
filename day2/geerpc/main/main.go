package main

import (
	"fmt"
	"geerpc"
	"log"
	"net"
	"sync"
)

func main() {
	addr := make(chan string)
	go func(addr chan string) {
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
			args := fmt.Sprintf("args: %d", i)
			var reply string
			if err := client.Call("Foo.Bar", args, &reply); err != nil {
				log.Fatal("call Foo.Bar error: ", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
