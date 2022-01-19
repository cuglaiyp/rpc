package main

import (
	"fmt"
	"net"
	"tlv/codec"
)

type s struct {
}

func (receiver s) a() {

}

func main() {

	listen, err := net.Listen("tcp", ":9999")
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	conn, err := listen.Accept()
	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	dec := codec.NewDecoder(conn)
	var args Args
	err = dec.Decode(&args)
	fmt.Println(args)
	conn.Close()

}

type Args struct {
	Arg1  int64
	Arg2  string
	Inner Inner
	Arg4  uint8
}

type Inner struct {
	Arg3 int8
}
