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
	"unicode/utf8"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/pkg/stdlib"
)

var stdinReader = bufio.NewReader(os.Stdin)

var Builtins = map[string]*object.Builtin{
	// __native_module__ resolves a registered native module (web, data,
	// money, security, realtime, ...) by name at runtime. The bytecode
	// compiler emits a call to this builtin for `import "mod" as alias`
	// because native module hashes contain Go closures that cannot be
	// serialized into a NABC bytecode image.
	"__native_module__": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `__native_module__`. got=%d, want=1", len(args))
			}
			name, ok := args[0].(*object.String)
			if !ok {
				return newError("argument to `__native_module__` must be STRING, got %s", args[0].Type())
			}
			if mod, ok := GetNativeModule(name.Value); ok {
				return mod
			}
			return newError("unknown native module %q", name.Value)
		},
	},
	"emit": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("wrong number of arguments to `emit`. got=%d, want=1 or 2", len(args))
			}
			if len(args) == 2 {
				return args[1]
			}
			return NULL
		},
	},
	"Channel": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `Channel`. got=%d, want=1", len(args))
			}
			capacity, ok := args[0].(*object.Integer)
			if !ok || capacity.Value < 0 {
				return newError("channel capacity must be a non-negative integer")
			}
			return object.NewChannel(int(capacity.Value))
		},
	},
	"send": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to `send`. got=%d, want=2", len(args))
			}
			channel, ok := args[0].(*object.Channel)
			if !ok {
				return newError("first argument to `send` must be CHANNEL, got %s", args[0].Type())
			}
			channel.Values <- args[1]
			return NULL
		},
	},
	"receive": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `receive`. got=%d, want=1", len(args))
			}
			channel, ok := args[0].(*object.Channel)
			if !ok {
				return newError("argument to `receive` must be CHANNEL, got %s", args[0].Type())
			}
			return <-channel.Values
		},
	},
	"len": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *object.String:
				return &object.Integer{Value: int64(utf8.RuneCountInString(arg.Value))}
			case *object.Array:
				return &object.Integer{Value: int64(arg.Len())}
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
			if element, ok := arr.Get(0); ok {
				return element
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
			length := arr.Len()
			if length > 0 {
				element, _ := arr.Get(int64(length - 1))
				return element
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
			elements := arr.Snapshot()
			if len(elements) > 0 {
				newElements := make([]object.Object, len(elements)-1)
				copy(newElements, elements[1:])
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
			elements := arr.Snapshot()
			newElements := make([]object.Object, len(elements)+1)
			copy(newElements, elements)
			newElements[len(elements)] = args[1]
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
			elements := arr.Snapshot()
			newElements := make([]object.Object, len(elements)+1)
			copy(newElements, elements)
			newElements[len(elements)] = args[1]
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
			if _, ok := args[1].(object.Hashable); !ok {
				return runtimeError(object.CategoryTypeError, "unusable as hash key: %s", args[1].Type())
			}
			// Copy-on-write, like push/append above: `set` returns a new
			// hash and leaves the argument untouched. Building the copy
			// through Clone and SetPair keeps the two views of the hash —
			// lookup table and insertion order — in step: the old body
			// rebuilt only the table (and mutated the caller's hash to do
			// it), so a key added with `set` could be looked up but never
			// appeared in iteration or printing.
			out := hash.Clone()
			out.SetPair(args[1], args[2])
			return out
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
			for _, e := range arr.Snapshot() {
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
					if length <= 0 {
						return &object.String{Value: ""}
					}
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
			err := os.Remove(path)
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
	"Ok": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `Ok`. got=%d, want=1", len(args))
			}
			return &object.Result{IsOk: true, Value: args[0]}
		},
	},
	"Err": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `Err`. got=%d, want=1", len(args))
			}
			return &object.Result{IsOk: false, Error: args[0]}
		},
	},
	"Some": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `Some`. got=%d, want=1", len(args))
			}
			return &object.Optional{HasValue: true, Value: args[0]}
		},
	},
	"None": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Optional{HasValue: false}
		},
	},
	"unwrap": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `unwrap`. got=%d, want=1", len(args))
			}
			switch o := args[0].(type) {
			case *object.Result:
				if o.IsOk {
					return o.Value
				}
				errStr := "unknown"
				if o.Error != nil {
					errStr = o.Error.Inspect()
				}
				return newError("unwrap failed on Err: %s", errStr)
			case *object.Optional:
				if o.HasValue {
					return o.Value
				}
				return newError("unwrap failed on None")
			default:
				return newError("argument to `unwrap` must be RESULT or OPTIONAL, got %s", args[0].Type())
			}
		},
	},
	"unwrapOr": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to `unwrapOr`. got=%d, want=2", len(args))
			}
			switch o := args[0].(type) {
			case *object.Result:
				if o.IsOk {
					return o.Value
				}
				return args[1]
			case *object.Optional:
				if o.HasValue {
					return o.Value
				}
				return args[1]
			default:
				return newError("first argument to `unwrapOr` must be RESULT or OPTIONAL, got %s", args[0].Type())
			}
		},
	},
	"isOk": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isOk`. got=%d, want=1", len(args))
			}
			if r, ok := args[0].(*object.Result); ok && r.IsOk {
				return TRUE
			}
			return FALSE
		},
	},
	"isErr": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isErr`. got=%d, want=1", len(args))
			}
			if r, ok := args[0].(*object.Result); ok && !r.IsOk {
				return TRUE
			}
			return FALSE
		},
	},
	"errValue": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `errValue`. got=%d, want=1", len(args))
			}
			r, ok := args[0].(*object.Result)
			if !ok {
				return newError("`errValue` expects a RESULT, got %s", args[0].Type())
			}
			if r.IsOk {
				return newError("`errValue` on Ok returns nothing: the value is %s", r.Value.Inspect())
			}
			if r.Error == nil {
				return NULL
			}
			return r.Error
		},
	},
	"unreachable": {
		Fn: func(args ...object.Object) object.Object {
			// The message defaults to something that names the construct, so a
			// `match` that reaches its non-exhaustive case says so even if the
			// caller's own wording was lost.
			msg := "reached code that was expected to be unreachable"
			if len(args) > 0 {
				msg = args[0].Inspect()
			}
			if len(args) > 1 {
				return newError("wrong number of arguments to `unreachable`. got=%d, want=0..1", len(args))
			}
			return newError("%s", msg)
		},
	},
	"isSome": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isSome`. got=%d, want=1", len(args))
			}
			if o, ok := args[0].(*object.Optional); ok && o.HasValue {
				return TRUE
			}
			return FALSE
		},
	},
	"isNone": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isNone`. got=%d, want=1", len(args))
			}
			if o, ok := args[0].(*object.Optional); ok && !o.HasValue {
				return TRUE
			}
			return FALSE
		},
	},
	"isNull": {
		// `??` and `?.` lower to a test against null, so both engines need this
		// name. It is exported for the same reason `isSome` is: a developer
		// writing `x ?? d` should be able to write `isNull(x)` when they want the
		// question on its own.
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to `isNull`. got=%d, want=1", len(args))
			}
			if object.IsNull(args[0]) {
				return TRUE
			}
			return FALSE
		},
	},
	"native": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 {
				return newError("wrong number of arguments to `native`. got=%d, want at least 1", len(args))
			}
			sym, ok := args[0].(*object.String)
			if !ok {
				return newError("first argument to `native` must be STRING symbol name, got %s", args[0].Type())
			}
			res, err := stdlib.CallHost(sym.Value, args[1:])
			if err != nil {
				return newError("native '%s' error: %s", sym.Value, err)
			}
			return res
		},
	},
}
