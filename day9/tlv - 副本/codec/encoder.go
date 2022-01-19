package codec

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"reflect"
	"tlv/core"
)

type Encoder struct {
	writer io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{writer: w}
}

func (e *Encoder) EncodeStruct(key core.Kind, valueBytes []byte) []byte {
	pkg := core.Pkg{
		Tag: core.Tag{
			FrameType: core.FrameTypePrimitive,
			DataType:  core.DataTypeStruct,
			TagValue:  key,
		},
		Value: valueBytes,
	}
	return pkg.Bytes()
}

func (e *Encoder) EncodePrimitive(key core.Kind, valueBytes []byte) []byte {
	pkg := core.Pkg{
		Tag: core.Tag{
			FrameType: core.FrameTypePrimitive,
			DataType:  core.DataTypePrimitive,
			TagValue:  key,
		},
		Value: valueBytes,
	}
	return pkg.Bytes()
}

func (e *Encoder) EncodeString(value string) ([]byte, error) {
	valueBytes := []byte(value)
	return e.EncodePrimitive(core.String, valueBytes), nil
}

func (e *Encoder) EncodeBool(value bool) ([]byte, error) {
	valueBytes := []byte{0}
	if value {
		valueBytes[0] = 1
	}
	return e.EncodePrimitive(core.Bool, valueBytes), nil
}

func (e *Encoder) EncodeInt8(value int8) ([]byte, error) {
	return e.encodeUint8(core.Int8, uint8(value))
}

func (e *Encoder) EncodeUint8(value uint8) ([]byte, error) {
	return e.encodeUint8(core.Uint8, value)
}

func (e *Encoder) encodeUint8(key core.Kind, value uint8) ([]byte, error) {
	valueBytes := []byte{value}
	return e.EncodePrimitive(key, valueBytes), nil
}

func (e *Encoder) EncodeInt16(value int16) ([]byte, error) {
	return e.encodeUint16(core.Int16, uint16(value))
}

func (e *Encoder) EncodeUint16(value uint16) ([]byte, error) {
	return e.encodeUint16(core.Uint16, value)
}

func (e *Encoder) encodeUint16(key core.Kind, value uint16) ([]byte, error) {
	valueBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(valueBytes, value)
	return e.EncodePrimitive(key, valueBytes), nil
}

func (e *Encoder) EncodeInt32(value int32) ([]byte, error) {
	return e.encodeUint32(core.Int32, uint32(value))
}

func (e *Encoder) EncodeUint32(value uint32) ([]byte, error) {
	return e.encodeUint32(core.Uint32, value)
}

func (e *Encoder) encodeUint32(key core.Kind, value uint32) ([]byte, error) {
	valueBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(valueBytes, value)
	return e.EncodePrimitive(key, valueBytes), nil
}

func (e *Encoder) EncodeInt64(value int64) ([]byte, error) {
	return e.encodeUint64(core.Int64, uint64(value))
}

func (e *Encoder) EncodeUint64(value uint64) ([]byte, error) {
	return e.encodeUint64(core.Uint64, value)
}

func (e *Encoder) encodeUint64(key core.Kind, value uint64) ([]byte, error) {
	valueBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(valueBytes, value)
	return e.EncodePrimitive(key, valueBytes), nil
}

func (e *Encoder) EncodeInt(value int) ([]byte, error) {
	return e.EncodeVarInt(int64(value))
}

func (e *Encoder) EncodeUint(value uint) ([]byte, error) {
	return e.EncodeVarUint(uint64(value))
}

// EncodeVarInt 写入任意长度的整形数据，范围为int8到int64之间，根据数值大小，自动计算
func (e *Encoder) EncodeVarInt(value int64) ([]byte, error) {
	var bytes []byte
	var err error
	if math.MinInt8 <= value && value <= math.MaxInt8 {
		bytes, err = e.EncodeInt8(int8(value))
	} else if math.MinInt16 <= value && value <= math.MaxInt16 {
		bytes, err = e.EncodeInt16(int16(value))
	} else if math.MinInt32 <= value && value <= math.MaxInt32 {
		bytes, err = e.EncodeInt32(int32(value))
	} else {
		bytes, err = e.EncodeInt64(int64(value))
	}
	return bytes, err
}

func (e *Encoder) EncodeVarUint(value uint64) ([]byte, error) {
	var bytes []byte
	var err error
	if 0 <= value && value <= math.MaxUint8 {
		bytes, err = e.EncodeUint8(uint8(value))
	} else if math.MaxUint8 < value && value <= math.MaxUint16 {
		bytes, err = e.EncodeUint16(uint16(value))
	} else if math.MaxUint16 < value && value <= math.MaxUint32 {
		bytes, err = e.EncodeUint32(uint32(value))
	} else {
		bytes, err = e.EncodeUint64(uint64(value))
	}
	return bytes, err
}

func (e *Encoder) EncodeObj(input interface{}) ([]byte, error) {
	inVal := reflect.Indirect(reflect.ValueOf(input))
	kind := inVal.Kind()
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.EncodeVarInt(inVal.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return e.EncodeVarUint(inVal.Uint())
	case reflect.String:
		return e.EncodeString(inVal.String())
	case reflect.Struct:
		numField := inVal.NumField()
		var innerBytes []byte
		for i := 0; i < numField; i++ {
			bytes, _ := e.EncodeObj(inVal.Field(i).Interface())
			innerBytes = append(innerBytes, bytes...)
		}
		res := e.EncodeStruct(1, innerBytes)
		return res, nil
	default:
		return nil, fmt.Errorf("不支持的类型：%s", kind.String())
	}
}

func (e *Encoder) Encode(input interface{}) error {
	byts, err := e.EncodeObj(input)
	if err != nil {
		log.Printf("写入失败")
		return err
	}


	if _, err = e.writer.Write(byts); err != nil {
		log.Printf("写入失败")
		return err
	}
	return nil
}
