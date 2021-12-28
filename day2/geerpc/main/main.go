package main

import (
	"encoding/json"
	"fmt"
	"geerpc"
	"geerpc/codec"
	"log"
	"net"
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

	conn, err := net.Dial("tcp", <-addr)
	if err != nil {
		log.Println("客户端连接失败")
		return
	}

	json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	cc := codec.NewGobCodec(conn)
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.BAr",
			Seq:           uint64(i),
			Error:         "",
		}
		body := "hello server"
		cc.Write(h, body)
		var reply string
		cc.ReadHeader(h)
		cc.ReadBody(&reply)
		fmt.Println(reply)
	}
}