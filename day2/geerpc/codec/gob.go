package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)
// 1. 在 codec.go 中
//    - 我们抽象了编解码器的接口 Codec，设计好了 Header（Body 目前还不知道存放什么东西，先放着）
//    - 定义了编解码器的种类，以及存放编解码器构造函数的容器
// 2. 在 gob.go 中
//    - 我们定义了一个具体的编解码器：GobCodec，实现了 Codec
//    - 实现了该编解码器的构造函数



// 由于是 GobCodec，我们放到一个新文件中

// GobCodec 该定义 Codec 具体的实现类了
// 由于只用了 Gob 这个类型，所以先只定义 GobCodec
type GobCodec struct {
	// 编解码器首先得有编解码的类
	enc *gob.Encoder // 编码器
	dec *gob.Decoder // 解码器
	// 编解码谁呢？连接！
	conn io.ReadWriteCloser // 连接，所以这个连接会放入上面两个编解码器中
	// 上面三个字段其实已经够了。但是为了效率高一点，我们引入一个带缓冲 Writer，这个 Writer 会给 enc 使用
	buf  *bufio.Writer

}

func (g *GobCodec) Close() error {
	return g.conn.Close()
}

func (g *GobCodec) ReadHeader(header *Header) error {
	// 用解码器把 conn（在构造的时候conn已经被放入了 dec）里面的头读出来
	return g.dec.Decode(header)
}

func (g *GobCodec) ReadBody(body interface{}) error {
	// 同上
	return g.dec.Decode(body)
}

func (g *GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil { // 判断写入的过程有没有发生错误，有的话，关闭连接
			_ = g.conn.Close()
		}
	}()
	// 写方法就需要编码器，进行写入了
	if er := g.enc.Encode(header); er != nil {
		log.Println("GobCodec: 写入 header 失败：", er) // 出现 error 的地方，我们记录一下
		return er
	}
	// 上下两种写法是等价的，两个 错误 都是新的变量，在 return 的时候会被写到 返回值 err 中，然后再执行 defer
	if err := g.enc.Encode(body); err != nil {
		log.Println("GobCodec: 写入 body 失败：", err)
		return err
	}
	// 最后，因为 enc 里面使用的是带缓冲的 Writer，所以需要 flush 一下
	// g.buf.Flush() 在 go 里面我们可以把这个操作放到 defer 里去做
	return nil
}

// NewGobCodec 类型定义好了，接着可以定义他的构造函数了
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf: buf,
		enc: gob.NewEncoder(buf), // enc 是编码器，用来写的，所以我们把 buf 丢进去，以提高效率
		dec: gob.NewDecoder(conn),
	}
}

var _ Codec = &GobCodec{}

