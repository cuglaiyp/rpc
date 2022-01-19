package codec

import (
	"io"
	"log"
	tlv "tlv/codec"
)

type TlvCodec struct {
	// 编解码器首先得有编解码的类
	enc *tlv.Encoder // 编码器
	dec *tlv.Decoder // 解码器
	// 编解码谁呢？连接！
	conn io.ReadWriteCloser // 连接，所以这个连接会放入上面两个编解码器中
}

func (t *TlvCodec) Close() error {
	return t.conn.Close()
}

func (g *TlvCodec) ReadHeader(header *Header) error {
	// 用解码器把 conn（在构造的时候conn已经被放入了 dec）里面的头读出来
	return g.dec.Decode(header)
}

func (g *TlvCodec) ReadBody(body interface{}) error {
	// 同上
	return g.dec.Decode(body)
}

func (g *TlvCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
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

// NewTlvCodec 类型定义好了，接着可以定义他的构造函数了
func NewTlvCodec(conn io.ReadWriteCloser) Codec {
	return &TlvCodec{
		conn: conn,
		enc:  tlv.NewEncoder(conn), // enc 是编码器，用来写的，所以我们把 buf 丢进去，以提高效率
		dec:  tlv.NewDecoder(conn),
	}
}

var _ Codec = &TlvCodec{}