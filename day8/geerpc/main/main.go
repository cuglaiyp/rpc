package main

import (
	"context"
	"geerpc"
	"geerpc/registry"
	"geerpc/xclient"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

func main() {
	log.SetFlags(log.Lmsgprefix)
	registryAddr := "localhost:2181"
	/*var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()
	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)*/

	call(registryAddr)
	broadcast(registryAddr)
	time.Sleep(time.Second * 5)
	call(registryAddr)
	broadcast(registryAddr)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	listener, _ := net.Listen("tcp", ":0")
	server := geerpc.NewServer()
	server.Register(new(Foo))
	//registry.Heartbeat(registryAddr, "tcp@" + listener.Addr().String(), 0)
	//registry.PutZkServer(listener.Addr().String())
	zk := registry.NewZkClient(registryAddr, 30*time.Second)
	zk.PutServer(listener.Addr().String())
	wg.Done()
	server.Accept(listener)
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func call(registry string) {
	//discovery := xclient.NewGeeRegistryDiscovery(registry, 0)
	discovery := xclient.NewZkRegistryDiscovery("localhost:2181", 30*time.Second)
	xc := xclient.NewXClient(discovery, xclient.RandomSelect, nil)
	defer xc.Close()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{
				Num1: i,
				Num2: i * i,
			}
			foo(xc, context.Background(), "call", "Foo.Sum", args)
		}(i)
	}
	wg.Wait()
}

func broadcast(registry string) {
	discovery := xclient.NewZkRegistryDiscovery("localhost:2181", 30*time.Second)
	xc := xclient.NewXClient(discovery, xclient.RandomSelect, nil)
	defer xc.Close()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{
				Num1: i,
				Num2: i * i,
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
