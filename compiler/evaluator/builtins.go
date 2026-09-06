package evaluator

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	"append": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if args[0].Type() != object.ARRAY_OBJ {
				return newError("argument to `append` must be ARRAY, got %s", args[0].Type())
			}
			arr := args[0].(*object.Array)
			length := len(arr.Elements)
			newElements := make([]object.Object, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]
			return &object.Array{Elements: newElements}
		},
	},
	"set": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("wrong number of arguments to `set`. got=%d, want=3", len(args))
			}
			hash, ok := args[0].(*object.Hash)
			if !ok {
				return newError("first argument to `set` must be HASH, got %s", args[0].Type())
			}
			hashable, ok := args[1].(object.Hashable)
			if !ok {
				return newError("unusable as hash key: %s", args[1].Type())
			}
			newPairs := make(map[object.HashKey]object.HashPair)
			for k, v := range hash.Pairs {
				newPairs[k] = v
			}
			newPairs[hashable.HashKey()] = object.HashPair{Key: args[1], Value: args[2]}
			hash.Pairs = newPairs
			return hash
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
			if len(args) == 1 {
				return &object.String{Value: strings.Trim(args[0].Inspect(), " \t\r\n\xef\xbb\xbf\ufeff")}
			}
			if len(args) == 2 {
				return &object.String{Value: strings.Trim(args[0].Inspect(), args[1].Inspect())}
			}
			return newError("wrong number of arguments. got=%d, want=1 or 2", len(args))
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
	"hasSuffix": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			if strings.HasSuffix(args[0].Inspect(), args[1].Inspect()) {
				return TRUE
			}
			return FALSE
		},
	},
	"replace": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 {
				return newError("wrong number of arguments. got=%d, want=3", len(args))
			}
			return &object.String{Value: strings.ReplaceAll(args[0].Inspect(), args[1].Inspect(), args[2].Inspect())}
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
	"exec": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 {
				return newError("wrong number of arguments to `exec`. got=0, want>=1")
			}
			cmdName := args[0].Inspect()
			var cmdArgs []string
			for _, a := range args[1:] {
				cmdArgs = append(cmdArgs, a.Inspect())
			}

			var cmd *exec.Cmd
			if len(args) == 1 && strings.Contains(cmdName, " ") && runtime.GOOS == "windows" {
				cmd = exec.Command("cmd.exe", "/c", cmdName)
			} else {
				cmd = exec.Command(cmdName, cmdArgs...)
			}

			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					return &object.Integer{Value: int64(exitErr.ExitCode())}
				}
				return &object.Integer{Value: 1}
			}
			return &object.Integer{Value: 0}
		},
	},
	"readFile": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `readFile`. got=%d, want=1", len(args))
			}
			filePath := args[0].Inspect()
			data, err := os.ReadFile(filePath)
			if err != nil {
				return newError("readFile error: %v", err)
			}
			content := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
			return &object.String{Value: content}
		},
	},
	"writeFile": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to `writeFile`. got=%d, want=2", len(args))
			}
			filePath := args[0].Inspect()
			content := args[1].Inspect()
			err := os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				return FALSE
			}
			return TRUE
		},
	},
	"fileExists": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `fileExists`. got=%d, want=1", len(args))
			}
			path := args[0].Inspect()
			_, err := os.Stat(path)
			if err == nil {
				return TRUE
			}
			return FALSE
		},
	},
	"isDir": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isDir`. got=%d, want=1", len(args))
			}
			path := args[0].Inspect()
			fi, err := os.Stat(path)
			if err == nil && fi.IsDir() {
				return TRUE
			}
			return FALSE
		},
	},
	"listDir": {
		Fn: func(args ...object.Object) object.Object {
			path := "."
			if len(args) > 0 && args[0].Inspect() != "" {
				path = args[0].Inspect()
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return &object.Array{Elements: []object.Object{}}
			}
			var elems []object.Object
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				elems = append(elems, &object.String{Value: name})
			}
			return &object.Array{Elements: elems}
		},
	},
	"makeDir": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `makeDir`. got=%d, want=1", len(args))
			}
			path := args[0].Inspect()
			err := os.MkdirAll(path, 0755)
			if err != nil {
				return FALSE
			}
			return TRUE
		},
	},
	"removeFile": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `removeFile`. got=%d, want=1", len(args))
			}
			path := args[0].Inspect()
			err := os.RemoveAll(path)
			if err != nil {
				return FALSE
			}
			return TRUE
		},
	},
	"cwd": {
		Fn: func(args ...object.Object) object.Object {
			dir, err := os.Getwd()
			if err != nil {
				return &object.String{Value: "."}
			}
			return &object.String{Value: dir}
		},
	},
	"chdir": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `chdir`. got=%d, want=1", len(args))
			}
			path := args[0].Inspect()
			err := os.Chdir(path)
			if err != nil {
				return FALSE
			}
			return TRUE
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
