package main

import (
	"context"
	"geerpc"
	"geerpc/xclient"
	"log"
	"net"
	"sync"
	"time"
)

func main() {
	log.SetFlags(log.Lmsgprefix)
	startServer()
}

func startServer() {
	listener, _ := net.Listen("tcp", "localhost:9998")
	server := geerpc.NewServer()
	server.Register(new(Foo))
	server.Accept(listener)
}


func call(registry string) {
	discovery := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(discovery, xclient.RandomSelect, nil)
	defer xc.Close()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{
				Num1: i,
				Num2: i*i,
			}
			foo(xc, context.Background(), "call", "Foo.Sum", args)
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string) {
	discovery := xclient.NewGeeRegistryDiscovery(registry, 0)
	xc := xclient.NewXClient(discovery, xclient.RandomSelect, nil)
	defer xc.Close()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{
				Num1: i,
				Num2: i*i,
			}
			foo(xc, context.Background(), "broadcast", "Foo.Sum", args)
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}

}

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}
