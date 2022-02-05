package parser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/hdt3213/rdb/lzf"
	"math"
	"strconv"
)

const (
	len6Bit      = 0
	len14Bit     = 1
	len32or64Bit = 2
	lenSpecial   = 3
	len32Bit     = 0x80
	len64Bit     = 0x81

	encodeInt8  = 0
	encodeInt16 = 1
	encodeInt32 = 2
	encodeLZF   = 3
)

// readLength parse Length Encoding
// see: https://github.com/sripathikrishnan/redis-rdb-tools/wiki/Redis-RDB-Dump-File-Format#length-encoding
func (p *Parser) readLength() (uint64, bool, error) {
	firstByte, err := p.readByte()
	if err != nil {
		return 0, false, fmt.Errorf("read length failed: %v", err)
	}
	lenType := (firstByte & 0xc0) >> 6 // get first 2 bits
	var length uint64
	special := false
	switch lenType {
	case len6Bit:
		length = uint64(firstByte) & 0x3f
	case len14Bit:
		nextByte, err := p.readByte()
		if err != nil {
			return 0, false, fmt.Errorf("read len14Bit failed: %v", err)
		}
		length = (uint64(firstByte)&0x3f)<<8 | uint64(nextByte)
	case len32or64Bit:
		if firstByte == len32Bit {
			err = p.readFull(p.buffer[0:4])
			if err != nil {
				return 0, false, fmt.Errorf("read len32Bit failed: %v", err)
			}
			length = uint64(binary.BigEndian.Uint32(p.buffer))
		} else if firstByte == len64Bit {
			err = p.readFull(p.buffer)
			if err != nil {
				return 0, false, fmt.Errorf("read len64Bit failed: %v", err)
			}
			length = binary.BigEndian.Uint64(p.buffer)
		} else {
			return 0, false, fmt.Errorf("illegal length encoding: %x", firstByte)
		}
	case lenSpecial:
		special = true
		length = uint64(firstByte) & 0x3f
	}
	return length, special, nil
}

func (p *Parser) readString() ([]byte, error) {
	length, special, err := p.readLength()
	if err != nil {
		return nil, err
	}

	if special {
		switch length {
		case encodeInt8:
			b, err := p.readByte()
			return []byte(strconv.Itoa(int(b))), err
		case encodeInt16:
			b, err := p.readUint16()
			return []byte(strconv.Itoa(int(b))), err
		case encodeInt32:
			b, err := p.readUint32()
			return []byte(strconv.Itoa(int(b))), err
		case encodeLZF:
			return p.readLZF()
		default:
			return []byte{}, errors.New("Unknown string encode type ")
		}
	}

	res := make([]byte, length)
	err = p.readFull(res)
	return res, err
}

func (p *Parser) readUint16() (uint16, error) {
	err := p.readFull(p.buffer[:2])
	if err != nil {
		return 0, fmt.Errorf("read uint16 error: %v", err)
	}

	i := binary.LittleEndian.Uint16(p.buffer[:2])
	return i, nil
}

func (p *Parser) readUint32() (uint32, error) {
	err := p.readFull(p.buffer[:4])
	if err != nil {
		return 0, fmt.Errorf("read uint16 error: %v", err)
	}

	i := binary.LittleEndian.Uint32(p.buffer[:4])
	return i, nil
}

func (p *Parser) readLiteralFloat() (float64, error) {
	first, err := p.readByte()
	if err != nil {
		return 0, err
	}
	if first == 0xff {
		return math.Inf(-1), nil
	} else if first == 0xfe {
		return math.Inf(1), nil
	} else if first == 0xfd {
		return math.NaN(), nil
	}
	buf := make([]byte, first)
	err = p.readFull(buf)
	if err != nil {
		return 0, err
	}
	str := string(buf)
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("")
	}
	return val, err
}

func (p *Parser) readFloat() (float64, error) {
	err := p.readFull(p.buffer)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(p.buffer)
	return math.Float64frombits(bits), nil
}

func (p *Parser) readLZF() ([]byte, error) {
	inLen, _, err := p.readLength()
	if err != nil {
		return nil, err
	}
	outLen, _, err := p.readLength()
	if err != nil {
		return nil, err
	}
	val := make([]byte, inLen)
	err = p.readFull(val)
	if err != nil {
		return nil, err
	}
	result := lzf.Decompress(val, int(inLen), int(outLen))
	return result, nil
}
