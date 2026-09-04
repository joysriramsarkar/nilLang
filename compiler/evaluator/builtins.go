package evaluator

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var stdinReader = bufio.NewReader(os.Stdin)

var Builtins = map[string]*object.Builtin{
	"len": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.String:
				return &object.Integer{Value: int64(len(arg.Value))}
			case *object.Array:
				return &object.Integer{Value: int64(len(arg.Elements))}
			default:
				return newError("argument to `len` not supported, got %s", args[0].Type())
			}
		},
	},
	"print": {
		Fn: func(args ...object.Object) object.Object {
			parts := []string{}
			for _, arg := range args {
				parts = append(parts, arg.Inspect())
			}
			fmt.Print(strings.Join(parts, " "))
			return NULL
		},
	},
	"println": {
		Fn: func(args ...object.Object) object.Object {
			parts := []string{}
			for _, arg := range args {
				parts = append(parts, arg.Inspect())
			}
			fmt.Println(strings.Join(parts, " "))
			return NULL
		},
	},
	"puts": {
		Fn: func(args ...object.Object) object.Object {
			parts := []string{}
			for _, arg := range args {
				parts = append(parts, arg.Inspect())
			}
			fmt.Println(strings.Join(parts, " "))
			return NULL
		},
	},
	"type": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: string(args[0].Type())}
		},
	},
	"first": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `first` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			if len(arr.Elements) > 0 {
				return arr.Elements[0]
			}
			return NULL
		},
	},
	"last": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `last` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				return arr.Elements[length-1]
			}
			return NULL
		},
	},
	"rest": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `rest` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			if length > 0 {
				newElements := make([]object.Object, length-1)
				copy(newElements, arr.Elements[1:length])
				return &object.Array{Elements: newElements}
			}
			return NULL
		},
	},
	"push": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `push` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			newElements := make([]object.Object, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]
			return &object.Array{Elements: newElements}
		},
	},
	"str": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: args[0].Inspect()}
		},
	},
	"int": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.Integer:
				return arg
			case *object.Float:
				return &object.Integer{Value: int64(arg.Value)}
			case *object.String:
				val, err := strconv.ParseInt(arg.Value, 0, 64)
				if err != nil {
					return newError("could not parse %q as integer", arg.Value)
				}
				return &object.Integer{Value: val}
			default:
				return newError("argument to `int` not supported, got %s", args[0].Type())
			}
		},
	},
	"time": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Integer{Value: time.Now().Unix()}
		},
	},
	"assert": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 {
				return newError("wrong number of arguments. got=%d, want>=1", len(args))
			}
			cond := isTruthy(args[0])
			if !cond {
				msg := "assertion failed"
				if len(args) > 1 {
					msg = args[1].Inspect()
				}
				return newError("%s", msg)
			}
			return TRUE
		},
	},
	"lower": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: strings.ToLower(args[0].Inspect())}
		},
	},
	"upper": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: strings.ToUpper(args[0].Inspect())}
		},
	},
	"trim": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &object.String{Value: strings.Trim(args[0].Inspect(), " \t\r\n\xef\xbb\xbf\ufeff")}
		},
	},
	"split": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("wrong number of arguments. got=%d, want=1 or 2", len(args))
			}
			sep := " "
			if len(args) == 2 {
				sep = args[1].Inspect()
			}
			parts := strings.Split(args[0].Inspect(), sep)
			var elems []object.Object
			for _, p := range parts {
				elems = append(elems, &object.String{Value: p})
			}
			return &object.Array{Elements: elems}
		},
	},
	"join": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*object.Array)
			if !ok {
				return newError("first argument to `join` must be ARRAY, got %s", args[0].Type())
			}
			sep := args[1].Inspect()
			var parts []string
			for _, e := range arr.Elements {
				parts = append(parts, e.Inspect())
			}
			return &object.String{Value: strings.Join(parts, sep)}
		},
	},
	"contains": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if strings.Contains(args[0].Inspect(), args[1].Inspect()) {
				return TRUE
			}
			return FALSE
		},
	},
	"hasPrefix": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if strings.HasPrefix(args[0].Inspect(), args[1].Inspect()) {
				return TRUE
			}
			return FALSE
		},
	},
	"substr": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
			}
			str := args[0].Inspect()
			runes := []rune(str)
			start := 0
			if intObj, ok := args[1].(*object.Integer); ok {
				start = int(intObj.Value)
			}
			if start < 0 {
				start = 0
			}
			if start > len(runes) {
				return &object.String{Value: ""}
			}
			end := len(runes)
			if len(args) == 3 {
				if lenObj, ok := args[2].(*object.Integer); ok {
					length := int(lenObj.Value)
					if start+length < end {
						end = start + length
					}
				}
			}
			return &object.String{Value: string(runes[start:end])}
		},
	},
	"clear": {
		Fn: func(args ...object.Object) object.Object {
			fmt.Print("\033[H\033[2J")
			return NULL
		},
	},
	"input": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) > 0 {
				fmt.Print(args[0].Inspect())
			}
			text, err := stdinReader.ReadString('\n')
			if err != nil && len(text) == 0 {
				return NULL
			}
			return &object.String{Value: strings.TrimRight(text, "\r\n")}
		},
	},
	"exit": {
		Fn: func(args ...object.Object) object.Object {
			code := 0
			if len(args) > 0 {
				if intObj, ok := args[0].(*object.Integer); ok {
					code = int(intObj.Value)
				}
			}
			os.Exit(code)
			return NULL
		},
	},
}
