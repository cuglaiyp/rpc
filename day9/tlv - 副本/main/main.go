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

	/*listen, err := net.Listen("tcp", ":9999")
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	conn, err := listen.Accept()

	if err != nil {
		fmt.Printf(err.Error())
		return
	}*/

	conn, err := net.Dial("tcp", ":9999")
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	args := Args{
		Arg2: "sds ",
		Arg4: 2,
	}
	/*args2 := Args{
		Arg1: 33,
		Arg2: "sdffd ",
		Inner: &Inner{
			Arg3: 23,
		},
		Arg4: 2,
	}*/
	/*var args Args
	decoder := codec.NewDecoder(conn)
	decoder.Decode(&args)*/
	encoder := codec.NewEncoder(conn)
	encoder.Encode(args)
	//encoder.Encode(args2)
	conn.Close()
}

type Args struct {
	Arg1  int32
	Arg2  string
	Inner *Inner
	Arg4  uint8
}

type Inner struct {
	Arg3 int8
}
