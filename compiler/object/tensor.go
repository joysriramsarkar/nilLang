package object

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
)

const TENSOR_OBJ = "TENSOR"

const tensorMatMulParallelWork = 256 * 1024

type TensorDType string

const (
	TensorFloat64 TensorDType = "float64"
	TensorFloat32 TensorDType = "float32"
	TensorInt32   TensorDType = "int32"
)

type Tensor struct {
	Data  []float64
	Shape []int
	DType TensorDType
}

func NewTensor(data []float64, shape []int) (*Tensor, error) {
	return NewTensorWithDType(data, shape, TensorFloat64)
}

func NewTensorWithDType(data []float64, shape []int, dtype TensorDType) (*Tensor, error) {
	size := 1
	for _, dimension := range shape {
		if dimension < 0 {
			return nil, fmt.Errorf("tensor dimensions must be non-negative")
		}
		size *= dimension
	}
	if len(shape) == 0 {
		size = 1
	}
	if size != len(data) {
		return nil, fmt.Errorf("tensor shape requires %d values, got %d", size, len(data))
	}
	quantized, err := quantizeTensorData(data, dtype)
	if err != nil {
		return nil, err
	}
	return &Tensor{Data: quantized, Shape: append([]int(nil), shape...), DType: dtype}, nil
}

func (tensor *Tensor) Cast(dtype TensorDType) (*Tensor, error) {
	return NewTensorWithDType(tensor.Data, tensor.Shape, dtype)
}

func (tensor *Tensor) Type() ObjectType { return TENSOR_OBJ }

func (tensor *Tensor) Inspect() string {
	dimensions := make([]string, len(tensor.Shape))
	for index, dimension := range tensor.Shape {
		dimensions[index] = fmt.Sprintf("%d", dimension)
	}
	values := make([]string, len(tensor.Data))
	for index, value := range tensor.Data {
		values[index] = fmt.Sprintf("%g", value)
	}
	return fmt.Sprintf("tensor(shape=[%s], data=[%s])", strings.Join(dimensions, ", "), strings.Join(values, ", "))
}

func (tensor *Tensor) At(indices []int) (float64, error) {
	if len(indices) != len(tensor.Shape) {
		return 0, fmt.Errorf("tensor index rank %d does not match rank %d", len(indices), len(tensor.Shape))
	}
	offset := 0
	stride := 1
	for dimension := len(tensor.Shape) - 1; dimension >= 0; dimension-- {
		index := indices[dimension]
		if index < 0 || index >= tensor.Shape[dimension] {
			return 0, fmt.Errorf("tensor index %d is out of bounds for dimension %d", index, dimension)
		}
		offset += index * stride
		stride *= tensor.Shape[dimension]
	}
	return tensor.Data[offset], nil
}

func (tensor *Tensor) Slice(starts, ends []int) (*Tensor, error) {
	if len(starts) != len(tensor.Shape) || len(ends) != len(tensor.Shape) {
		return nil, fmt.Errorf("tensor slice bounds must match rank %d", len(tensor.Shape))
	}
	shape := make([]int, len(tensor.Shape))
	for dimension := range tensor.Shape {
		if starts[dimension] < 0 || starts[dimension] > ends[dimension] || ends[dimension] > tensor.Shape[dimension] {
			return nil, fmt.Errorf("tensor slice [%d:%d] is invalid for dimension %d with size %d", starts[dimension], ends[dimension], dimension, tensor.Shape[dimension])
		}
		shape[dimension] = ends[dimension] - starts[dimension]
	}
	size := tensorSize(shape)
	data := make([]float64, size)
	for outputOffset := range data {
		remaining := outputOffset
		inputOffset := 0
		inputStride := 1
		for dimension := len(shape) - 1; dimension >= 0; dimension-- {
			coordinate := remaining % shape[dimension]
			remaining /= shape[dimension]
			inputOffset += (starts[dimension] + coordinate) * inputStride
			inputStride *= tensor.Shape[dimension]
		}
		data[outputOffset] = tensor.Data[inputOffset]
	}
	return NewTensorWithDType(data, shape, tensor.DType)
}

func TensorAdd(left, right *Tensor) (*Tensor, error) {
	return tensorElementwise(left, right, func(leftValue, rightValue float64) float64 {
		return leftValue + rightValue
	})
}

func TensorMul(left, right *Tensor) (*Tensor, error) {
	return tensorElementwise(left, right, func(leftValue, rightValue float64) float64 {
		return leftValue * rightValue
	})
}

func tensorElementwise(left, right *Tensor, operation func(float64, float64) float64) (*Tensor, error) {
	shape, err := broadcastShape(left.Shape, right.Shape)
	if err != nil {
		return nil, err
	}
	size := tensorSize(shape)
	data := make([]float64, size)
	for index := range data {
		data[index] = operation(left.Data[broadcastOffset(index, shape, left.Shape)], right.Data[broadcastOffset(index, shape, right.Shape)])
	}
	return NewTensorWithDType(data, shape, promoteTensorDType(left.DType, right.DType))
}

func TensorDot(left, right *Tensor) (float64, error) {
	if len(left.Shape) != 1 || len(right.Shape) != 1 || left.Shape[0] != right.Shape[0] {
		return 0, fmt.Errorf("tensor dot requires vectors with matching lengths")
	}
	result := 0.0
	for index := range left.Data {
		result += left.Data[index] * right.Data[index]
	}
	return result, nil
}

func TensorMatMul(left, right *Tensor) (*Tensor, error) {
	if len(left.Shape) != 2 || len(right.Shape) != 2 {
		return nil, fmt.Errorf("tensor matmul requires two rank-2 tensors")
	}
	rows, shared, columns := left.Shape[0], left.Shape[1], right.Shape[1]
	if shared != right.Shape[0] {
		return nil, fmt.Errorf("tensor matmul dimensions do not align: %d and %d", shared, right.Shape[0])
	}
	data := make([]float64, rows*columns)
	workerCount := tensorMatMulWorkerCount(rows, columns, shared)
	if workerCount == 1 {
		tensorMatMulRows(left, right, data, 0, rows)
		return NewTensorWithDType(data, []int{rows, columns}, promoteTensorDType(left.DType, right.DType))
	}
	rowsPerWorker := (rows + workerCount - 1) / workerCount
	var workers sync.WaitGroup
	for startRow := 0; startRow < rows; startRow += rowsPerWorker {
		endRow := min(startRow+rowsPerWorker, rows)
		workers.Add(1)
		go func(startRow, endRow int) {
			defer workers.Done()
			tensorMatMulRows(left, right, data, startRow, endRow)
		}(startRow, endRow)
	}
	workers.Wait()
	return NewTensorWithDType(data, []int{rows, columns}, promoteTensorDType(left.DType, right.DType))
}

func tensorMatMulRows(left, right *Tensor, output []float64, startRow, endRow int) {
	shared, columns := left.Shape[1], right.Shape[1]
	for row := startRow; row < endRow; row++ {
		for inner := 0; inner < shared; inner++ {
			leftValue := left.Data[row*shared+inner]
			for column := 0; column < columns; column++ {
				output[row*columns+column] += leftValue * right.Data[inner*columns+column]
			}
		}
	}
}

func tensorMatMulWorkerCount(rows, columns, shared int) int {
	if rows < 2 || columns == 0 || shared == 0 || rows < tensorMatMulParallelWork/columns/shared {
		return 1
	}
	return min(runtime.GOMAXPROCS(0), rows)
}

func sameShape(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func broadcastShape(left, right []int) ([]int, error) {
	rank := max(len(left), len(right))
	shape := make([]int, rank)
	for outputDimension := rank - 1; outputDimension >= 0; outputDimension-- {
		leftDimension := alignedDimension(left, outputDimension, rank)
		rightDimension := alignedDimension(right, outputDimension, rank)
		if leftDimension != rightDimension && leftDimension != 1 && rightDimension != 1 {
			return nil, fmt.Errorf("tensor shapes %v and %v cannot be broadcast", left, right)
		}
		shape[outputDimension] = leftDimension
		if leftDimension == 1 {
			shape[outputDimension] = rightDimension
		}
	}
	return shape, nil
}

func alignedDimension(shape []int, outputDimension, outputRank int) int {
	inputDimension := outputDimension - (outputRank - len(shape))
	if inputDimension < 0 {
		return 1
	}
	return shape[inputDimension]
}

func tensorSize(shape []int) int {
	size := 1
	for _, dimension := range shape {
		size *= dimension
	}
	return size
}

func quantizeTensorData(data []float64, dtype TensorDType) ([]float64, error) {
	quantized := make([]float64, len(data))
	for index, value := range data {
		switch dtype {
		case TensorFloat64:
			quantized[index] = value
		case TensorFloat32:
			quantized[index] = float64(float32(value))
		case TensorInt32:
			integerValue := math.Trunc(value)
			if math.IsNaN(value) || math.IsInf(value, 0) || integerValue < math.MinInt32 || integerValue > math.MaxInt32 {
				return nil, fmt.Errorf("tensor value %g at index %d cannot be represented as int32", value, index)
			}
			quantized[index] = integerValue
		default:
			return nil, fmt.Errorf("unsupported tensor dtype %q", dtype)
		}
	}
	return quantized, nil
}

func promoteTensorDType(left, right TensorDType) TensorDType {
	if left == TensorFloat64 || right == TensorFloat64 {
		return TensorFloat64
	}
	if left == TensorFloat32 || right == TensorFloat32 {
		return TensorFloat32
	}
	return TensorInt32
}

func broadcastOffset(outputOffset int, outputShape, inputShape []int) int {
	inputOffset := 0
	inputStride := 1
	for outputDimension := len(outputShape) - 1; outputDimension >= 0; outputDimension-- {
		coordinate := outputOffset % outputShape[outputDimension]
		outputOffset /= outputShape[outputDimension]
		inputDimension := outputDimension - (len(outputShape) - len(inputShape))
		if inputDimension >= 0 {
			if inputShape[inputDimension] != 1 {
				inputOffset += coordinate * inputStride
			}
			inputStride *= inputShape[inputDimension]
		}
	}
	return inputOffset
}
