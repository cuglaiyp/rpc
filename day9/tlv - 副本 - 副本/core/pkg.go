package core

import (
	"fmt"
)

type Tag struct {
	FrameType byte //帧类型，0-基本类型，1-私有类型
	DataType  byte //数据类型，0-基本数据，1-TLV数据
	TagValue  Kind //tag类型值
}

type Pkg struct {
	tagByteCount  int //类型字段占用的字节数
	lenByteCount  int //长度字段占用的字节数
	dataByteCount int //数据字段占用的字节数

	Tag Tag

	Value []byte //实际数据

	data []byte //数据包字节数据
}

// encode 将 Pkg 编码成 byte 数组
func (p *Pkg) build() {
	p.dataByteCount = len(p.Value)

	tagBytes := buildTag(p.Tag.FrameType, p.Tag.DataType, p.Tag.TagValue)
	lenBytes := buildLength(p.dataByteCount)

	p.tagByteCount = len(tagBytes)
	p.lenByteCount = len(lenBytes)

	p.data = append(p.data, tagBytes...)
	p.data = append(p.data, lenBytes...)
	p.data = append(p.data, p.Value...)
}

// Size 获取TLV数据包大小
func (p *Pkg) Size() int {
	return p.tagByteCount + p.lenByteCount + p.dataByteCount
}

// Bytes 获取TLV数据包的字节数据
func (p *Pkg) Bytes() []byte {
	if p.data == nil {
		p.build()
	}
	return p.data
}

func (p *Pkg) String() (ret string) {
	ret = fmt.Sprintf("FrameType = %d, DataType = %d, TagValue = %d, Vaule = %v\n", p.Tag.FrameType, p.Tag.DataType, p.Tag.TagValue, p.Value)
	return ret
}

// 生成TLV的Tag字节数据
func buildTag(frameType byte, dataType byte, tagValue Kind) (tagBytes []byte) {

	tagValueBytes := buildLength(int(tagValue))

	if tagValue > 0x1f {
		tagBytes = append(tagBytes, 0x80)
	}
	tagBytes = append(tagBytes, tagValueBytes...)

	tagBytes[0] = tagBytes[0] | frameType | dataType

	return tagBytes
}

// 生成TLV的数据长度字节数据
func buildLength(length int) (lenBytes []byte) {

	if length < 0 {
		fmt.Errorf("长度不能为负数 length = %d\n", length)
	}

	if length == 0 {
		return []byte{0}
	}

	for {
		if length <= 0 {
			break
		}
		digit := length % 128
		length = length / 128
		if length > 0 {
			digit = digit | 0x80
		}

		lenBytes = append(lenBytes, byte(digit))
	}
	return lenBytes
}
