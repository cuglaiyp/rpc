package core

import "strconv"

type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Ptr
	Slice
	String
	Struct
	UnsafePointer
)

// String returns the name of k.
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "kind" + strconv.Itoa(int(k))
}

var kindNames = []string{
	Invalid:       "invalid",
	Bool:          "bool",
	Int:           "int",
	Int8:          "int8",
	Int16:         "int16",
	Int32:         "int32",
	Int64:         "int64",
	Uint:          "uint",
	Uint8:         "uint8",
	Uint16:        "uint16",
	Uint32:        "uint32",
	Uint64:        "uint64",
	Uintptr:       "uintptr",
	Float32:       "float32",
	Float64:       "float64",
	Complex64:     "complex64",
	Complex128:    "complex128",
	Array:         "array",
	Chan:          "chan",
	Func:          "func",
	Interface:     "interface",
	Map:           "map",
	Ptr:           "ptr",
	Slice:         "slice",
	String:        "string",
	Struct:        "struct",
	UnsafePointer: "unsafe.Pointer",
}

var IntKinds = map[Kind]struct{}{
	Int:   {},
	Int8:  {},
	Int16: {},
	Int32: {},
	Int64: {},
	Uint:   {},
	Uint8:  {},
	Uint16: {},
	Uint32: {},
	Uint64: {},
}


// 帧类型
const (
	FrameTypePrimitive = 0x00 //基本类型   0000 0000
	FrameTypePrivate   = 0x40 //私有类型   0100 0000
)

// 数据类型
const (
	DataTypePrimitive = 0x00 //基本数据编码   0000 0000
	DataTypeStruct    = 0x20 //TLV嵌套       0010 0000
)
