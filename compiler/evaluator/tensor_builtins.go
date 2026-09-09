package evaluator

import "github.com/joysriramsarkar/nilLang/compiler/object"

func init() {
	Builtins["tensor"] = &object.Builtin{Fn: tensorBuiltin}
	Builtins["tensorShape"] = &object.Builtin{Fn: tensorShapeBuiltin}
	Builtins["tensorGet"] = &object.Builtin{Fn: tensorGetBuiltin}
	Builtins["tensorAdd"] = &object.Builtin{Fn: tensorAddBuiltin}
	Builtins["tensorMul"] = &object.Builtin{Fn: tensorMulBuiltin}
	Builtins["tensorSlice"] = &object.Builtin{Fn: tensorSliceBuiltin}
	Builtins["tensorDtype"] = &object.Builtin{Fn: tensorDtypeBuiltin}
	Builtins["tensorCast"] = &object.Builtin{Fn: tensorCastBuiltin}
	Builtins["tensorDot"] = &object.Builtin{Fn: tensorDotBuiltin}
	Builtins["tensorMatmul"] = &object.Builtin{Fn: tensorMatmulBuiltin}
	Builtins["tensorSum"] = &object.Builtin{Fn: tensorSumBuiltin}
}

func tensorBuiltin(args ...object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return newError("wrong number of arguments to `tensor`. got=%d, want=2 or 3", len(args))
	}
	data, errObject := numericArray(args[0], "tensor data")
	if errObject != nil {
		return errObject
	}
	shapeValues, errObject := integerArray(args[1], "tensor shape")
	if errObject != nil {
		return errObject
	}
	dtype := object.TensorFloat64
	if len(args) == 3 {
		dtypeValue, ok := args[2].(*object.String)
		if !ok {
			return newError("tensor dtype must be STRING, got %s", args[2].Type())
		}
		dtype = object.TensorDType(dtypeValue.Value)
	}
	tensor, err := object.NewTensorWithDType(data, shapeValues, dtype)
	if err != nil {
		return newError("%s", err)
	}
	return tensor
}

func tensorDtypeBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 1, 0, "tensorDtype")
	if errObject != nil {
		return errObject
	}
	return &object.String{Value: string(tensor.DType)}
}

func tensorCastBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 2, 0, "tensorCast")
	if errObject != nil {
		return errObject
	}
	dtypeValue, ok := args[1].(*object.String)
	if !ok {
		return newError("tensor cast dtype must be STRING, got %s", args[1].Type())
	}
	result, err := tensor.Cast(object.TensorDType(dtypeValue.Value))
	if err != nil {
		return newError("%s", err)
	}
	return result
}

func tensorShapeBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 1, 0, "tensorShape")
	if errObject != nil {
		return errObject
	}
	elements := make([]object.Object, len(tensor.Shape))
	for index, dimension := range tensor.Shape {
		elements[index] = &object.Integer{Value: int64(dimension)}
	}
	return &object.Array{Elements: elements}
}

func tensorGetBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 2, 0, "tensorGet")
	if errObject != nil {
		return errObject
	}
	indices, errObject := integerArray(args[1], "tensor indices")
	if errObject != nil {
		return errObject
	}
	value, err := tensor.At(indices)
	if err != nil {
		return newError("%s", err)
	}
	return &object.Float{Value: value}
}

func tensorAddBuiltin(args ...object.Object) object.Object {
	left, right, errObject := tensorPair(args, "tensorAdd")
	if errObject != nil {
		return errObject
	}
	result, err := object.TensorAdd(left, right)
	if err != nil {
		return newError("%s", err)
	}
	return result
}

func tensorMulBuiltin(args ...object.Object) object.Object {
	left, right, errObject := tensorPair(args, "tensorMul")
	if errObject != nil {
		return errObject
	}
	result, err := object.TensorMul(left, right)
	if err != nil {
		return newError("%s", err)
	}
	return result
}

func tensorSliceBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 3, 0, "tensorSlice")
	if errObject != nil {
		return errObject
	}
	starts, errObject := integerArray(args[1], "tensor slice starts")
	if errObject != nil {
		return errObject
	}
	ends, errObject := integerArray(args[2], "tensor slice ends")
	if errObject != nil {
		return errObject
	}
	result, err := tensor.Slice(starts, ends)
	if err != nil {
		return newError("%s", err)
	}
	return result
}

func tensorDotBuiltin(args ...object.Object) object.Object {
	left, right, errObject := tensorPair(args, "tensorDot")
	if errObject != nil {
		return errObject
	}
	result, err := object.TensorDot(left, right)
	if err != nil {
		return newError("%s", err)
	}
	return &object.Float{Value: result}
}

func tensorMatmulBuiltin(args ...object.Object) object.Object {
	left, right, errObject := tensorPair(args, "tensorMatmul")
	if errObject != nil {
		return errObject
	}
	result, err := object.TensorMatMul(left, right)
	if err != nil {
		return newError("%s", err)
	}
	return result
}

func tensorSumBuiltin(args ...object.Object) object.Object {
	tensor, errObject := tensorArgument(args, 1, 0, "tensorSum")
	if errObject != nil {
		return errObject
	}
	result := 0.0
	for _, value := range tensor.Data {
		result += value
	}
	return &object.Float{Value: result}
}

func tensorPair(args []object.Object, name string) (*object.Tensor, *object.Tensor, object.Object) {
	left, errObject := tensorArgument(args, 2, 0, name)
	if errObject != nil {
		return nil, nil, errObject
	}
	right, ok := args[1].(*object.Tensor)
	if !ok {
		return nil, nil, newError("argument 2 to `%s` must be TENSOR, got %s", name, args[1].Type())
	}
	return left, right, nil
}

func tensorArgument(args []object.Object, count, index int, name string) (*object.Tensor, object.Object) {
	if len(args) != count {
		return nil, newError("wrong number of arguments to `%s`. got=%d, want=%d", name, len(args), count)
	}
	tensor, ok := args[index].(*object.Tensor)
	if !ok {
		return nil, newError("argument %d to `%s` must be TENSOR, got %s", index+1, name, args[index].Type())
	}
	return tensor, nil
}

func numericArray(value object.Object, label string) ([]float64, object.Object) {
	array, ok := value.(*object.Array)
	if !ok {
		return nil, newError("%s must be ARRAY, got %s", label, value.Type())
	}
	values := make([]float64, len(array.Elements))
	for index, element := range array.Elements {
		switch element := element.(type) {
		case *object.Integer:
			values[index] = float64(element.Value)
		case *object.Float:
			values[index] = element.Value
		default:
			return nil, newError("%s element %d must be numeric, got %s", label, index, element.Type())
		}
	}
	return values, nil
}

func integerArray(value object.Object, label string) ([]int, object.Object) {
	array, ok := value.(*object.Array)
	if !ok {
		return nil, newError("%s must be ARRAY, got %s", label, value.Type())
	}
	values := make([]int, len(array.Elements))
	for index, element := range array.Elements {
		integer, ok := element.(*object.Integer)
		if !ok {
			return nil, newError("%s element %d must be INTEGER, got %s", label, index, element.Type())
		}
		values[index] = int(integer.Value)
	}
	return values, nil
}
