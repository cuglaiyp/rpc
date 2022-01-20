package codec

import (
	"encoding/binary"
	"errors"
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

func (d *Decoder) Decode(res interface{}) error {
	resValue := reflect.ValueOf(res)
	if resValue.Kind() != reflect.Ptr {
		return errors.New("只能是指针类型")
	}

	tag, length := d.readTL()
	n, err := io.ReadFull(d.reader, d.buf[d.offset:d.offset+length])
	if n < length || err != nil {
		return err
	}
	d.decode0(d.buf[d.offset:], tag, reflect.Indirect(resValue))
	return nil
}

func (d *Decoder) readTL() (*core.Tag, int) {
	var tag *core.Tag
	var length int

	for {
		n, _ := io.ReadFull(d.reader, d.buf[d.offset:d.offset+1])
		if n == 0 {
			return nil, 0
		}
		if d.buf[d.offset]&0x80 == 0 {
			d.offset++
			tag = parseTag(d.buf[0:d.offset])
			break
		}
		d.offset++
	}
	lenStart := d.offset
	for {
		n, _ := io.ReadFull(d.reader, d.buf[len(d.buf):1])
		if n == 0 {
			return nil, 0
		}
		if d.buf[d.offset]&0x80 == 0 {
			d.offset++
			length = parseLength(d.buf[lenStart:d.offset])
			break
		}
		d.offset++
	}
	return tag, length
}

func (d *Decoder) DecodeBytes(data []byte, res interface{}) {
	resValue := reflect.ValueOf(res)
	if resValue.Kind() != reflect.Ptr {
		return
	}
	d.decode(data, reflect.Indirect(resValue))
}

func (d *Decoder) decode(data []byte, resElemValue reflect.Value) int {
	isFindTag := false
	tag := &core.Tag{}
	lenStart, valueStart, valueLen := 0, 0, 0
	curIdx := 0
	for ; curIdx < len(data); curIdx++ {
		//计算tag
		if isFindTag == false {
			if data[curIdx]&0x80 == 0 {
				isFindTag = true
				lenStart = curIdx + 1
				tag = parseTag(data[0:lenStart])
			}
			continue
		}

		//计算length
		if data[curIdx]&0x80 == 0 {
			valueStart = curIdx + 1
			valueLen = parseLength(data[lenStart:valueStart])
			curIdx = valueStart + valueLen
			if curIdx > len(data) {
				log.Println("数据不完整")
				return 0
			}
			// 解析 value
			d.decode0(data[valueStart:curIdx], tag, resElemValue)
			break
		}
	}
	return curIdx
}

func (d *Decoder) decode0(data []byte, tag *core.Tag, resElemValue reflect.Value) {
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
			resElemValue.Set(reflect.ValueOf(int16(binary.BigEndian.Uint16(data))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint16:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint16(data)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int32:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			resElemValue.Set(reflect.ValueOf(int32(binary.BigEndian.Uint32(data))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint32:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint32(data)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int64:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val, data)
			resElemValue.Set(reflect.ValueOf(int64(binary.BigEndian.Uint64(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint64:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val, data)
			resElemValue.Set(reflect.ValueOf(binary.BigEndian.Uint64(val)))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Int:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val, data)
			resElemValue.Set(reflect.ValueOf(int(binary.BigEndian.Uint64(val))))
		} else {
			log.Printf("目标类型[%s]与编码类型[%s]不匹配！\n", kind.String(), tag.TagValue.String())
		}
	case reflect.Uint:
		if _, ok := core.IntKinds[tag.TagValue]; ok {
			val := make([]byte, 8)
			copy(val, data)
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
		preIdx := 0
		for i := 0; i < filedNums; i++ {
			fieldi := resElemValue.Field(i)
			preIdx += d.decode(data[preIdx:], fieldi)
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

func (d *Decoder) Read(output interface{}) {

}
