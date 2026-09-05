package wasm

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// WasmModule represents a compiled WebAssembly module
type WasmModule struct {
	WAT    string
	Binary []byte
}

// Compiler translates MIR programs to WebAssembly
type Compiler struct{}

func NewCompiler() *Compiler {
	return &Compiler{}
}

// CompileToWAT compiles a MIR program to WebAssembly Text (WAT) format
func (c *Compiler) CompileToWAT(prog *mir.Program) (string, error) {
	var b strings.Builder
	b.WriteString("(module\n")
	b.WriteString("  (import \"env\" \"puts\" (func $puts (param i32)))\n")
	b.WriteString("  (memory (export \"memory\") 1)\n\n")

	if prog.Main != nil {
		b.WriteString("  (func $main (export \"main\") (result i64)\n")
		b.WriteString("    (local $temp i64)\n")

		for _, block := range prog.Main.Blocks {
			b.WriteString(fmt.Sprintf("    ;; block %s\n", block.ID))
			for _, inst := range block.Instructions {
				watInst := c.lowerInstructionToWAT(inst)
				if watInst != "" {
					b.WriteString("    ")
					b.WriteString(watInst)
					b.WriteByte('\n')
				}
			}
			if block.Terminator != nil {
				watTerm := c.lowerTerminatorToWAT(block.Terminator)
				if watTerm != "" {
					b.WriteString("    ")
					b.WriteString(watTerm)
					b.WriteByte('\n')
				}
			}
		}

		b.WriteString("  )\n")
	}

	b.WriteString(")\n")
	return b.String(), nil
}

func (c *Compiler) lowerInstructionToWAT(inst mir.Instruction) string {
	switch i := inst.(type) {
	case mir.StoreVarInst:
		return fmt.Sprintf(";; store %s", i.Name)
	case mir.LoadVarInst:
		return fmt.Sprintf(";; load %s", i.Name)
	case mir.BinaryOpInst:
		left := c.operandToWAT(i.Left)
		right := c.operandToWAT(i.Right)
		op := "i64.add"
		switch i.Op {
		case "+":
			op = "i64.add"
		case "-":
			op = "i64.sub"
		case "*":
			op = "i64.mul"
		case "/":
			op = "i64.div_s"
		}
		return fmt.Sprintf("%s\n    %s\n    %s", left, right, op)
	default:
		return ""
	}
}

func (c *Compiler) lowerTerminatorToWAT(term mir.Terminator) string {
	switch t := term.(type) {
	case mir.ReturnTerminator:
		if t.Value != nil {
			return c.operandToWAT(t.Value) + "\n    return"
		}
		return "i64.const 0\n    return"
	default:
		return ""
	}
}

func (c *Compiler) operandToWAT(op mir.Operand) string {
	switch o := op.(type) {
	case mir.ConstOperand:
		switch v := o.Value.(type) {
		case int64:
			return fmt.Sprintf("i64.const %d", v)
		case int:
			return fmt.Sprintf("i64.const %d", v)
		default:
			return "i64.const 0"
		}
	case mir.VarOperand:
		return "i64.const 0"
	case mir.TempOperand:
		return ""
	default:
		return "i64.const 0"
	}
}

// CompileToBinary generates standard WebAssembly binary (.wasm)
func (c *Compiler) CompileToBinary(prog *mir.Program) ([]byte, error) {
	var buf bytes.Buffer

	// 1. WASM Magic Number: \0asm (0x00, 0x61, 0x73, 0x6d)
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d})
	// 2. WASM Version: 1
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00})

	// 3. Type Section (ID = 1): () -> i64
	typeSec := encodeSection(1, func(b *bytes.Buffer) {
		writeVarUint(b, 1) // 1 type entry
		b.WriteByte(0x60)  // func type
		writeVarUint(b, 0) // 0 params
		writeVarUint(b, 1) // 1 result
		b.WriteByte(0x7e)  // i64 (0x7E)
	})
	buf.Write(typeSec)

	// 4. Function Section (ID = 3): function index 0 has type 0
	funcSec := encodeSection(3, func(b *bytes.Buffer) {
		writeVarUint(b, 1) // 1 function
		writeVarUint(b, 0) // type index 0
	})
	buf.Write(funcSec)

	// 5. Export Section (ID = 7): export "main" funcidx 0
	exportSec := encodeSection(7, func(b *bytes.Buffer) {
		writeVarUint(b, 1)     // 1 export
		writeString(b, "main") // name "main"
		b.WriteByte(0x00)      // kind = func
		writeVarUint(b, 0)     // func index 0
	})
	buf.Write(exportSec)

	// 6. Code Section (ID = 10): Body of main function
	codeSec := encodeSection(10, func(b *bytes.Buffer) {
		writeVarUint(b, 1) // 1 function body

		var body bytes.Buffer
		writeVarUint(&body, 0) // 0 locals

		// If main has instructions, compile them
		if prog.Main != nil {
			for _, bb := range prog.Main.Blocks {
				for _, inst := range bb.Instructions {
					if binOp, ok := inst.(mir.BinaryOpInst); ok {
						emitOperandBin(&body, binOp.Left)
						emitOperandBin(&body, binOp.Right)
						switch binOp.Op {
						case "+":
							body.WriteByte(0x7c) // i64.add
						case "-":
							body.WriteByte(0x7d) // i64.sub
						case "*":
							body.WriteByte(0x7e) // i64.mul
						case "/":
							body.WriteByte(0x7f) // i64.div_s
						default:
							body.WriteByte(0x7c)
						}
					}
				}
				if ret, ok := bb.Terminator.(mir.ReturnTerminator); ok && ret.Value != nil {
					emitOperandBin(&body, ret.Value)
				}
			}
		} else {
			body.WriteByte(0x42) // i64.const
			writeVarInt(&body, 0)
		}

		body.WriteByte(0x0f) // return
		body.WriteByte(0x0b) // end

		// write body length and body
		writeVarUint(b, uint64(body.Len()))
		b.Write(body.Bytes())
	})
	buf.Write(codeSec)

	return buf.Bytes(), nil
}

func emitOperandBin(b *bytes.Buffer, op mir.Operand) {
	switch o := op.(type) {
	case mir.ConstOperand:
		switch v := o.Value.(type) {
		case int64:
			b.WriteByte(0x42) // i64.const
			writeVarInt(b, v)
		case int:
			b.WriteByte(0x42)
			writeVarInt(b, int64(v))
		default:
			b.WriteByte(0x42)
			writeVarInt(b, 0)
		}
	default:
		// Temporary or variable
	}
}

func encodeSection(id byte, writeContent func(b *bytes.Buffer)) []byte {
	var content bytes.Buffer
	writeContent(&content)

	var sec bytes.Buffer
	sec.WriteByte(id)
	writeVarUint(&sec, uint64(content.Len()))
	sec.Write(content.Bytes())
	return sec.Bytes()
}

func writeVarUint(b *bytes.Buffer, val uint64) {
	for {
		bByte := byte(val & 0x7F)
		val >>= 7
		if val != 0 {
			bByte |= 0x80
		}
		b.WriteByte(bByte)
		if val == 0 {
			break
		}
	}
}

func writeVarInt(b *bytes.Buffer, val int64) {
	more := true
	for more {
		bByte := byte(val & 0x7F)
		val >>= 7
		sign := bByte & 0x40
		if (val == 0 && sign == 0) || (val == -1 && sign != 0) {
			more = false
		} else {
			bByte |= 0x80
		}
		b.WriteByte(bByte)
	}
}

func writeString(b *bytes.Buffer, s string) {
	writeVarUint(b, uint64(len(s)))
	b.WriteString(s)
}

func (c *Compiler) Compile(prog *mir.Program) (*WasmModule, error) {
	wat, err := c.CompileToWAT(prog)
	if err != nil {
		return nil, err
	}
	bin, err := c.CompileToBinary(prog)
	if err != nil {
		return nil, err
	}
	return &WasmModule{
		WAT:    wat,
		Binary: bin,
	}, nil
}
