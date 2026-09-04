package evaluator

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
	case *ast.AssignStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Assign(node.Name.Value, val)
		return val
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.NullLiteral:
		return NULL
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.StringTemplate:
		return evalStringTemplate(node, env)
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IfExpression:
		return evalIfExpression(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		fn := &object.Function{Parameters: params, Env: env, Body: body}
		if node.Name != "" {
			env.Set(node.Name, fn)
		}
		return fn
	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)
	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	case *ast.DotExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		return evalDotExpression(left, node.Member.Value, env)
	case *ast.HashLiteral:
		return evalHashLiteral(node, env)
	case *ast.ComponentLiteral:
		return &object.String{Value: fmt.Sprintf("Component<%s>", node.Name.Value)}
	}

	return nil
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		result = Eval(ws.Body, env)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

func evalStringTemplate(st *ast.StringTemplate, env *object.Environment) object.Object {
	var sb strings.Builder
	for _, part := range st.Parts {
		if part.IsExpression {
			val := Eval(part.Expression, env)
			if isError(val) {
				return val
			}
			sb.WriteString(val.Inspect())
		} else {
			sb.WriteString(part.Literal)
		}
	}
	return &object.String{Value: sb.String()}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		if intObj, ok := right.(*object.Integer); ok && intObj.Value == 0 {
			return TRUE
		}
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return &object.Integer{Value: -right.Value}
	case *object.Float:
		return &object.Float{Value: -right.Value}
	default:
		return newError("unknown operator: -%s", right.Type())
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ || right.Type() == object.STRING_OBJ:
		if operator == "+" {
			return &object.String{Value: left.Inspect() + right.Inspect()}
		}
		if operator == "==" {
			return nativeBoolToBooleanObject(left.Inspect() == right.Inspect())
		}
		if operator == "!=" {
			return nativeBoolToBooleanObject(left.Inspect() != right.Inspect())
		}
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case operator == "&&":
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	case operator == "||":
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		return &object.Integer{Value: leftVal / rightVal}
	case "%":
		if rightVal == 0 {
			return newError("modulo by zero")
		}
		return &object.Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "&&":
		return nativeBoolToBooleanObject(leftVal != 0 && rightVal != 0)
	case "||":
		return nativeBoolToBooleanObject(leftVal != 0 || rightVal != 0)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object) object.Object {
	var leftVal, rightVal float64

	if left.Type() == object.INTEGER_OBJ {
		leftVal = float64(left.(*object.Integer).Value)
	} else {
		leftVal = left.(*object.Float).Value
	}

	if right.Type() == object.INTEGER_OBJ {
		rightVal = float64(right.(*object.Integer).Value)
	} else {
		rightVal = right.(*object.Float).Value
	}

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		return &object.Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if node.Value == "ui" {
		return makeUINamespace()
	}

	if builtin, ok := Builtins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: %s", node.Value)
}

func makeUINamespace() *object.Hash {
	return MakeHashObj(map[string]object.Object{
		"NewPage": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Page"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Page"},
					"title":   &object.String{Value: title},
					"content": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewNavigation": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				brand := "App"
				if len(args) > 0 {
					brand = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":  &object.String{Value: "Navigation"},
					"brand": &object.String{Value: brand},
					"items": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewDashboard": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Dashboard"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Dashboard"},
					"title":   &object.String{Value: title},
					"metrics": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewTable": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				headers := &object.Array{Elements: args}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Table"},
					"headers": headers,
					"rows":    &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewForm": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Form"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":   &object.String{Value: "Form"},
					"title":  &object.String{Value: title},
					"fields": &object.Array{Elements: []object.Object{}},
				})
			},
		},
	})
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		if intObj, ok := obj.(*object.Integer); ok {
			return intObj.Value != 0
		}
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		if paramIdx < len(args) {
			env.Set(param.Value, args[paramIdx])
		}
	}

	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s[%s]", left.Type(), index.Type())
	}
}

func evalStringIndexExpression(str, index object.Object) object.Object {
	strObj := str.(*object.String)
	idx := index.(*object.Integer).Value
	runes := []rune(strObj.Value)
	if idx < 0 || idx >= int64(len(runes)) {
		return NULL
	}
	return &object.String{Value: string(runes[idx])}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalHashLiteral(node *ast.HashLiteral, env *object.Environment) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("unusable as hash key: %s", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: pairs}
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalDotExpression(left object.Object, member string, env *object.Environment) object.Object {
	switch obj := left.(type) {
	case *object.Hash:
		strKey := &object.String{Value: member}
		if pair, ok := obj.Pairs[strKey.HashKey()]; ok {
			return pair.Value
		}

		for _, pair := range obj.Pairs {
			if strObj, ok := pair.Key.(*object.String); ok {
				if strings.EqualFold(strObj.Value, member) {
					return pair.Value
				}
			}
		}

		return getHashMethod(obj, member)

	default:
		return newError("property access .%s not supported on type %s", member, left.Type())
	}
}

func getHashMethod(hash *object.Hash, method string) object.Object {
	normMethod := strings.ToLower(method)
	switch normMethod {
	case "setnav":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					setHashKey(hash, "navigation", args[0])
				}
				return hash
			},
		}
	case "add":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					appendHashList(hash, "content", args[0])
				}
				return hash
			},
		}
	case "setfooter":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					setHashKey(hash, "footer", args[0])
				}
				return hash
			},
		}
	case "additem":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 2 {
					item := MakeHashObj(map[string]object.Object{
						"label": args[0],
						"path":  args[1],
					})
					appendHashList(hash, "items", item)
				}
				return hash
			},
		}
	case "addmetric":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 3 {
					metric := MakeHashObj(map[string]object.Object{
						"label": args[0],
						"value": args[1],
						"delta": args[2],
					})
					appendHashList(hash, "metrics", metric)
				}
				return hash
			},
		}
	case "addrow":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				row := &object.Array{Elements: args}
				appendHashList(hash, "rows", row)
				return hash
			},
		}
	case "addfield":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 3 {
					fld := MakeHashObj(map[string]object.Object{
						"label":       args[0],
						"name":        args[1],
						"placeholder": args[2],
					})
					appendHashList(hash, "fields", fld)
				}
				return hash
			},
		}
	default:
		return newError("unknown method %s on object", method)
	}
}

func setHashKey(hash *object.Hash, key string, val object.Object) {
	k := &object.String{Value: key}
	hash.Pairs[k.HashKey()] = object.HashPair{Key: k, Value: val}
}

func appendHashList(hash *object.Hash, listKey string, item object.Object) {
	k := &object.String{Value: listKey}
	hKey := k.HashKey()

	var arr *object.Array
	if pair, ok := hash.Pairs[hKey]; ok {
		if a, isArr := pair.Value.(*object.Array); isArr {
			arr = a
		}
	}

	if arr == nil {
		arr = &object.Array{Elements: []object.Object{}}
		hash.Pairs[hKey] = object.HashPair{Key: k, Value: arr}
	}

	arr.Elements = append(arr.Elements, item)
}

func MakeHashObj(m map[string]object.Object) *object.Hash {
	pairs := make(map[object.HashKey]object.HashPair)
	for k, v := range m {
		keyObj := &object.String{Value: k}
		pairs[keyObj.HashKey()] = object.HashPair{Key: keyObj, Value: v}
	}
	return &object.Hash{Pairs: pairs}
}
