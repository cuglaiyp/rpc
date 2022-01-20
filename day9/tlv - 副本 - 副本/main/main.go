package main

import (
	"errors"
	"io"
	"net"
)

type s struct {
}

func (receiver s) a() {

}

func main() {

	conn, err := net.Dial("tcp", "localhost:9999")
	if err != nil {
		return
	}

	/*args := Args{
		Arg1:  20,
		Arg2:  "aa",
		Inner: Inner{
			Arg3: 2,
		},
		Arg4:  34,





	}*/

}

type Decoder struct {
	countBuf []byte
	buf      []byte
	err      error
	r        io.Reader
}

func (dec *Decoder) decodeTypeSequence(isInterface bool) typeId {
	for dec.err == nil {
		if dec.buf.Len() == 0 {
			if !dec.recvMessage() {
				break
			}
		}
		// Receive a type id.
		id := dec.nextInt()
		if id >= 0 {
			// Value follows.
			return id
		}
		// Type definition for (-id) follows.
		dec.recvType(-id)
		// When decoding an interface, after a type there may be a
		// DelimitedValue still in the buffer. Skip its count.
		// (Alternatively, the buffer is empty and the byte count
		// will be absorbed by recvMessage.)
		if dec.buf.Len() > 0 {
			if !isInterface {
				dec.err = errors.New("extra data in buffer")
				break
			}
			dec.nextUint()
		}
	}
	return -1
}

func (dec *Decoder) nextInt() int64 {
	n, _, err := decodeUintReader(&dec.buf, dec.countBuf)
	if err != nil {
		dec.err = err
	}
	return toInt(n)
}

func (dec *Decoder) recvMessage(conn net.Conn) bool {
	// Read a count.
	nbytes, _, err := decodeUintReader(dec.r, dec.countBuf)
	if err != nil {
		return false
	}
	dec.readMessage(int(nbytes))
	return true
}

func (dec *Decoder) readMessage(nbytes int) {
	// Read the data
	dec.buf = make([]byte, nbytes)
	_, dec.err = io.ReadFull(dec.r, dec.buf)
	if dec.err == io.EOF {
		dec.err = io.ErrUnexpectedEOF
	}
}

func decodeUintReader(r io.Reader, buf []byte) (x uint64, width int, err error) {
	width = 1
	n, err := io.ReadFull(r, buf[0:width])
	if n == 0 {
		return
	}
	b := buf[0]
	if b <= 0x7f {
		return uint64(b), width, nil
	}
	return
}

type Args struct {
	Arg1  int64
	Arg2  string
	Inner Inner
	Arg4  uint8
}

type Inner struct {
	Arg3 int8
}
