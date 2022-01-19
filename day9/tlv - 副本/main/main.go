package main

import (
	"bufio"
	"fmt"
	"net"
	"tlv/codec"
)

type s struct {
}

func (receiver s) a() {

}

func main() {

	conn, err := net.Dial("tcp", "localhost:9999")
	if err != nil {
		return
	}

	args := Args{
		Arg1:  20,
		Arg2:  "aa",
		Inner: Inner{
			Arg3: 2,
		},
		Arg4:  34,
	}
	writer := bufio.NewWriter(conn)
	dec := codec.NewEncoder(writer)
	err = dec.Encode(args)
	writer.Flush()
	fmt.Errorf("")

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
