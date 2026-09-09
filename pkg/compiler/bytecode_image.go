package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	corecompiler "github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var bytecodeMagic = [4]byte{'N', 'A', 'B', 'C'}

const (
	bytecodeVersion uint16 = 1
	maxImageItems          = 1 << 24

	constantInteger  byte = 1
	constantFloat    byte = 2
	constantString   byte = 3
	constantFunction byte = 4
)

func EncodeBytecode(bytecode *corecompiler.Bytecode) ([]byte, error) {
	if bytecode == nil {
		return nil, fmt.Errorf("cannot encode nil bytecode")
	}

	var image bytes.Buffer
	image.Write(bytecodeMagic[:])
	if err := binary.Write(&image, binary.BigEndian, bytecodeVersion); err != nil {
		return nil, err
	}
	if err := writeBytes(&image, bytecode.Instructions); err != nil {
		return nil, fmt.Errorf("encode instructions: %w", err)
	}
	if err := writeLength(&image, len(bytecode.Constants)); err != nil {
		return nil, fmt.Errorf("encode constants: %w", err)
	}
	for index, constant := range bytecode.Constants {
		if err := encodeConstant(&image, constant); err != nil {
			return nil, fmt.Errorf("encode constant %d: %w", index, err)
		}
	}
	return image.Bytes(), nil
}

func DecodeBytecode(image []byte) (*corecompiler.Bytecode, error) {
	reader := bytes.NewReader(image)
	var magic [4]byte
	if _, err := io.ReadFull(reader, magic[:]); err != nil {
		return nil, fmt.Errorf("read NABC header: %w", err)
	}
	if magic != bytecodeMagic {
		return nil, fmt.Errorf("invalid NABC header")
	}

	var version uint16
	if err := binary.Read(reader, binary.BigEndian, &version); err != nil {
		return nil, fmt.Errorf("read NABC version: %w", err)
	}
	if version != bytecodeVersion {
		return nil, fmt.Errorf("unsupported NABC version %d", version)
	}

	instructions, err := readBytes(reader)
	if err != nil {
		return nil, fmt.Errorf("decode instructions: %w", err)
	}
	constantCount, err := readLength(reader)
	if err != nil {
		return nil, fmt.Errorf("decode constants: %w", err)
	}
	constants := make([]object.Object, constantCount)
	for index := range constants {
		constants[index], err = decodeConstant(reader)
		if err != nil {
			return nil, fmt.Errorf("decode constant %d: %w", index, err)
		}
	}
	if reader.Len() != 0 {
		return nil, fmt.Errorf("NABC image has %d trailing bytes", reader.Len())
	}
	return &corecompiler.Bytecode{Instructions: instructions, Constants: constants}, nil
}

func encodeConstant(writer io.Writer, constant object.Object) error {
	switch value := constant.(type) {
	case *object.Integer:
		if err := binary.Write(writer, binary.BigEndian, constantInteger); err != nil {
			return err
		}
		return binary.Write(writer, binary.BigEndian, value.Value)
	case *object.Float:
		if err := binary.Write(writer, binary.BigEndian, constantFloat); err != nil {
			return err
		}
		return binary.Write(writer, binary.BigEndian, math.Float64bits(value.Value))
	case *object.String:
		if err := binary.Write(writer, binary.BigEndian, constantString); err != nil {
			return err
		}
		return writeBytes(writer, []byte(value.Value))
	case *object.CompiledFunction:
		if err := binary.Write(writer, binary.BigEndian, constantFunction); err != nil {
			return err
		}
		if err := writeBytes(writer, value.Instructions); err != nil {
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, int32(value.NumLocals)); err != nil {
			return err
		}
		return binary.Write(writer, binary.BigEndian, int32(value.NumParameters))
	default:
		return fmt.Errorf("unsupported constant type %T", constant)
	}
}

func decodeConstant(reader io.Reader) (object.Object, error) {
	var kind byte
	if err := binary.Read(reader, binary.BigEndian, &kind); err != nil {
		return nil, err
	}
	switch kind {
	case constantInteger:
		var value int64
		if err := binary.Read(reader, binary.BigEndian, &value); err != nil {
			return nil, err
		}
		return &object.Integer{Value: value}, nil
	case constantFloat:
		var bits uint64
		if err := binary.Read(reader, binary.BigEndian, &bits); err != nil {
			return nil, err
		}
		return &object.Float{Value: math.Float64frombits(bits)}, nil
	case constantString:
		value, err := readBytes(reader)
		if err != nil {
			return nil, err
		}
		return &object.String{Value: string(value)}, nil
	case constantFunction:
		instructions, err := readBytes(reader)
		if err != nil {
			return nil, err
		}
		var locals, parameters int32
		if err := binary.Read(reader, binary.BigEndian, &locals); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, binary.BigEndian, &parameters); err != nil {
			return nil, err
		}
		if locals < 0 || parameters < 0 {
			return nil, fmt.Errorf("negative function metadata")
		}
		return &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     int(locals),
			NumParameters: int(parameters),
		}, nil
	default:
		return nil, fmt.Errorf("unknown constant type %d", kind)
	}
}

func writeBytes(writer io.Writer, value []byte) error {
	if err := writeLength(writer, len(value)); err != nil {
		return err
	}
	_, err := writer.Write(value)
	return err
}

func readBytes(reader io.Reader) ([]byte, error) {
	length, err := readLength(reader)
	if err != nil {
		return nil, err
	}
	value := make([]byte, length)
	_, err = io.ReadFull(reader, value)
	return value, err
}

func writeLength(writer io.Writer, length int) error {
	if length < 0 || length > maxImageItems {
		return fmt.Errorf("length %d exceeds NABC limit", length)
	}
	return binary.Write(writer, binary.BigEndian, uint32(length))
}

func readLength(reader io.Reader) (int, error) {
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return 0, err
	}
	if length > maxImageItems {
		return 0, fmt.Errorf("length %d exceeds NABC limit", length)
	}
	return int(length), nil
}
