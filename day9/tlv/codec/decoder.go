package codec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"tlv/core"
)

type Decoder struct {
	reader io.Reader
	buf    []byte
	offset int // read offset
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		reader: r,
		buf:    make([]byte, 1024),
		offset: 0,
	}
}

// Decode 解码器的顶层入口
func (d *Decoder) Decode(res interface{}) error {
	resValue := reflect.ValueOf(res)
	if resValue.Kind() != reflect.Ptr {
		return errors.New("只能是指针类型")
	}

	tag, length, err := d.readTL()
	if err != nil {
		return err
	}
	n, err := io.ReadFull(d.reader, d.buf[d.offset:d.offset+length])
	if n < length || err != nil {
		return err
	}
	decodeTlv(d.buf[d.offset:d.offset+length], tag, reflect.Indirect(resValue))
	d.reset()
	return nil
}

// DecodeBytes 解码器的顶层入口
func (d *Decoder) DecodeBytes(data []byte, res interface{}) {
	resValue := reflect.ValueOf(res)
	if resValue.Kind() != reflect.Ptr {
		return
	}
	decode(data, reflect.Indirect(resValue))
}



func (d *Decoder) readTL() (*core.Tag, int, error) {
	var tag *core.Tag
	var length int
	tagStart := d.offset
	for {
		n, err := io.ReadFull(d.reader, d.buf[d.offset:d.offset+1])
		if n == 0 || err != nil {
			return nil, 0, fmt.Errorf("从流中读取错误")
		}
		d.offset++
		if d.buf[d.offset-1]&0x80 == 0 {
			tag = parseTag(d.buf[tagStart:d.offset])
			break
		}
	}
	lenStart := d.offset
	for {
		n, err := io.ReadFull(d.reader, d.buf[d.offset:d.offset+1])
		if n == 0 || err != nil {
			return nil, 0, fmt.Errorf("从流中读取错误")
		}
		d.offset++
		if d.buf[d.offset-1]&0x80 == 0 {
			length = parseLength(d.buf[lenStart:d.offset])
			break
		}

	}
	return tag, length, nil
}



func decode(buf []byte, resElemValue reflect.Value) int {
	var tag *core.Tag
	var length int
	offset := 0
	tagStart := offset
	for {
		if buf[offset]&0x80 == 0 {
			offset++
			tag = parseTag(buf[tagStart:offset])
			break
		}
		offset++
	}
	lenStart := offset
	for {
		if buf[offset]&0x80 == 0 {
			offset++
			length = parseLength(buf[lenStart:offset])
			break
		}
		offset++
	}
	decodeTlv(buf[offset:offset+length], tag, resElemValue)
	return length + offset
}

func decodeTlv(data []byte, tag *core.Tag, resElemValue reflect.Value) {
	kind := resElemValue.Kind()
	switch kind {
	case reflect.Int8:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			resElemValue.Set(reflect.ValueOf(int8(data[0])))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint8:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			resElemValue.Set(reflect.ValueOf(data[0]))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int16:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 2)
			copy(val[2-len(data):], data)
			resElemValue.Set(reflect.ValueOf(int16(binary.BigEndian.Uint16(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint16:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 2)
			copy(val[2-len(data):], data)
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint16(val)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int32:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 4)
			copy(val[4-len(data):], data)
			resElemValue.Set(reflect.ValueOf(int32(binary.BigEndian.Uint32(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint32:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 4)
			copy(val[4-len(data):], data)
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint32(val)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int64:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val[8-len(data):], data)
			resElemValue.Set(reflect.ValueOf(int64(binary.BigEndian.Uint64(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint64:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val[8-len(data):], data)
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint64(val)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val[8-len(data):], data)
			resElemValue.Set(reflect.ValueOf(int(binary.BigEndian.Uint64(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val[8-len(data):], data)
			resElemValue.Set(reflect.ValueOf(uint(binary.BigEndian.Uint64(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.String:
		if tag.TagValue == core.String {
			resElemValue.Set(reflect.ValueOf(string(data)))
		}
	case reflect.Struct:
		if tag.DataType != core.DataTypeStruct {
			return
		}
		filedNums := resElemValue.NumField()
		length := 0
		for i := 0; i < filedNums; i++ {
			fieldi := resElemValue.Field(i)
			length += decode(data[length:], reflect.Indirect(fieldi))
		}
	default:
		log.Printf("tlv decoder: 未支持类型[%s]！\n", kind.String())
	}
}

func parseTag(tagBytes []byte) *core.Tag {
	frameType := tagBytes[0] & core.FrameTypePrivate
	dataType := tagBytes[0] & core.DataTypeStruct
	tagValue := 0
	byteCount := len(tagBytes)

	if byteCount == 1 {
		tagValue = int(tagBytes[0] & 0x1f)
		return &core.Tag{
			FrameType: frameType,
			DataType:  dataType,
			TagValue:  core.Kind(tagValue),
		}
	}
	power := 1
	for i := 1; i < byteCount; i++ {
		digit := tagBytes[i]
		tagValue += int(digit&0x7f) * power
		power *= 128
	}
	return &core.Tag{
		FrameType: frameType,
		DataType:  dataType,
		TagValue:  core.Kind(tagValue),
	}
}

func parseLength(lenBytes []byte) (length int) {
	length = 0
	power := 1
	byteCount := len(lenBytes)
	for i := 0; i < byteCount; i++ {
		digit := lenBytes[i]
		length += int(digit&0x7f) * power
		power *= 128
	}
	return length
}

func (d *Decoder) reset() {
	d.offset = 0
}
