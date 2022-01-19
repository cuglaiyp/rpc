package codec

import (
	"io"
)

// 1. 在 codec.go 中
//    - 我们抽象了编解码器的接口 Codec，设计好了 Header（Body 目前还不知道存放什么东西，先放着）
//    - 定义了编解码器的种类，以及存放编解码器构造函数的容器

// Header 将请求的服务抽象到 Header 这个结构中，给头中额外增加一个序列号和错误消息
// ServiceMethod 是服务名和方法名，通常与 Go 语言中的结构体和方法相映射。
// Seq 是请求的序号，也可以认为是某个请求的 ID，用来区分不同的请求。
// Error 是错误信息，客户端置为空，服务端如果如果发生错误，将错误信息置于 Error 中。
// 注意与 Http 协议的请求头区分开来
type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}

// Codec 接着抽象出 Codec 解码器接口，解码器就需要对 Header 进行解码
type Codec interface {
	io.Closer                                     // 继承一下，表明 Codec 是可关闭的
	ReadHeader(header *Header) error              // 读取客户端请求 Header 的信息，并写入 header
	ReadBody(body interface{}) error              // 读取客户端 Body 的信息，并写入 body
	Write(header *Header, body interface{}) error // 将 Header、Body 里面的信息写回客户端
}

// NewCodecFunc 定义好了 Codec ，接着定义 Codec 的构造函数类型
type NewCodecFunc func(closer io.ReadWriteCloser) Codec

// Type 类型，根据这个类型可以从 map 里面取对应的构造函数
type Type string

// 定义两个默认类型。这个例子中只实现了 Gob 的 Codec，所以只用到了 GobType
const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
	TlvType  Type = "application/tlv"
)

// NewCodecFuncMap 存放各种类型的构造函数
var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	// 初始化 map
	NewCodecFuncMap = map[Type]NewCodecFunc{
		GobType: NewGobCodec,
		TlvType: NewTlvCodec,
	}
}
