package object

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"testing"
)

func TestTensorAddBroadcasting(t *testing.T) {
	tests := []struct {
		name       string
		leftShape  []int
		leftData   []float64
		rightShape []int
		rightData  []float64
		wantShape  []int
		wantData   []float64
	}{
		{
			name:       "bias vector across rows",
			leftShape:  []int{2, 3},
			leftData:   []float64{1, 2, 3, 4, 5, 6},
			rightShape: []int{3},
			rightData:  []float64{10, 20, 30},
			wantShape:  []int{2, 3},
			wantData:   []float64{11, 22, 33, 14, 25, 36},
		},
		{
			name:       "outer dimensions",
			leftShape:  []int{2, 1},
			leftData:   []float64{1, 2},
			rightShape: []int{1, 3},
			rightData:  []float64{10, 20, 30},
			wantShape:  []int{2, 3},
			wantData:   []float64{11, 21, 31, 12, 22, 32},
		},
		{
			name:       "operand order is symmetric",
			leftShape:  []int{3},
			leftData:   []float64{10, 20, 30},
			rightShape: []int{2, 3},
			rightData:  []float64{1, 2, 3, 4, 5, 6},
			wantShape:  []int{2, 3},
			wantData:   []float64{11, 22, 33, 14, 25, 36},
		},
		{
			name:       "zero-sized dimension",
			leftShape:  []int{0, 3},
			leftData:   []float64{},
			rightShape: []int{1, 3},
			rightData:  []float64{1, 2, 3},
			wantShape:  []int{0, 3},
			wantData:   []float64{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, err := NewTensor(test.leftData, test.leftShape)
			if err != nil {
				t.Fatal(err)
			}
			right, err := NewTensor(test.rightData, test.rightShape)
			if err != nil {
				t.Fatal(err)
			}
			result, err := TensorAdd(left, right)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(result.Shape, test.wantShape) || !slices.Equal(result.Data, test.wantData) {
				t.Fatalf("TensorAdd()=%v %v, want %v %v", result.Shape, result.Data, test.wantShape, test.wantData)
			}
		})
	}
}

func TestTensorDTypes(t *testing.T) {
	defaultTensor := mustTensor(t, []float64{1.25}, []int{1})
	if defaultTensor.DType != TensorFloat64 {
		t.Fatalf("default dtype=%q, want float64", defaultTensor.DType)
	}
	float32Tensor, err := NewTensorWithDType([]float64{1.0 / 3.0}, []int{1}, TensorFloat32)
	if err != nil {
		t.Fatal(err)
	}
	if float32Tensor.Data[0] != float64(float32(1.0/3.0)) {
		t.Fatalf("float32 value=%g was not quantized", float32Tensor.Data[0])
	}
	int32Tensor, err := NewTensorWithDType([]float64{2.9, -2.9}, []int{2}, TensorInt32)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(int32Tensor.Data, []float64{2, -2}) {
		t.Fatalf("int32 data=%v, want truncation toward zero", int32Tensor.Data)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), float64(math.MaxInt32) + 1} {
		if _, err := NewTensorWithDType([]float64{value}, []int{1}, TensorInt32); err == nil {
			t.Fatalf("int32 tensor accepted %g", value)
		}
	}
	if _, err := NewTensorWithDType([]float64{1}, []int{1}, TensorDType("f32")); err == nil {
		t.Fatal("tensor accepted unsupported dtype")
	}
}

func TestTensorDTypePropagation(t *testing.T) {
	left, _ := NewTensorWithDType([]float64{16777216}, []int{1}, TensorFloat32)
	right, _ := NewTensorWithDType([]float64{1}, []int{1}, TensorFloat32)
	result, err := TensorAdd(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if result.DType != TensorFloat32 || result.Data[0] != 16777216 {
		t.Fatalf("float32 addition=%v dtype=%q, want quantized 16777216", result.Data, result.DType)
	}
	mixed, err := TensorMul(left, mustTensor(t, []float64{2}, []int{1}))
	if err != nil {
		t.Fatal(err)
	}
	if mixed.DType != TensorFloat64 {
		t.Fatalf("mixed dtype=%q, want float64", mixed.DType)
	}
	sliced, err := left.Slice([]int{0}, []int{1})
	if err != nil || sliced.DType != TensorFloat32 {
		t.Fatalf("slice dtype=%q err=%v, want float32", sliced.DType, err)
	}
	cast, err := left.Cast(TensorInt32)
	if err != nil || cast.DType != TensorInt32 {
		t.Fatalf("cast dtype=%q err=%v, want int32", cast.DType, err)
	}
	cast.Data[0] = 0
	if left.Data[0] == 0 {
		t.Fatal("Cast() result aliases source data")
	}
}

func TestTensorAddRejectsIncompatibleShapes(t *testing.T) {
	left, _ := NewTensor([]float64{1, 2, 3, 4, 5, 6}, []int{2, 3})
	right, _ := NewTensor([]float64{1, 2}, []int{2})
	if _, err := TensorAdd(left, right); err == nil {
		t.Fatal("TensorAdd() accepted incompatible shapes")
	}
}

func TestTensorMulBroadcasting(t *testing.T) {
	left, _ := NewTensor([]float64{1, 2, 3, 4, 5, 6}, []int{2, 3})
	right, _ := NewTensor([]float64{10, 20, 30}, []int{3})
	result, err := TensorMul(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Shape, []int{2, 3}) || !slices.Equal(result.Data, []float64{10, 40, 90, 40, 100, 180}) {
		t.Fatalf("TensorMul()=%v %v", result.Shape, result.Data)
	}
	if _, err := TensorMul(left, mustTensor(t, []float64{1, 2}, []int{2})); err == nil {
		t.Fatal("TensorMul() accepted incompatible shapes")
	}
}

func TestTensorSlice(t *testing.T) {
	tensor := mustTensor(t, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, []int{3, 4})
	tests := []struct {
		name      string
		starts    []int
		ends      []int
		wantShape []int
		wantData  []float64
	}{
		{"row and column window", []int{1, 1}, []int{3, 3}, []int{2, 2}, []float64{6, 7, 10, 11}},
		{"full copy", []int{0, 0}, []int{3, 4}, []int{3, 4}, tensor.Data},
		{"empty rows", []int{2, 0}, []int{2, 4}, []int{0, 4}, []float64{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := tensor.Slice(test.starts, test.ends)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(result.Shape, test.wantShape) || !slices.Equal(result.Data, test.wantData) {
				t.Fatalf("Slice()=%v %v, want %v %v", result.Shape, result.Data, test.wantShape, test.wantData)
			}
			if len(result.Data) > 0 {
				original := tensor.Data[0]
				result.Data[0] = -1
				if tensor.Data[0] != original {
					t.Fatal("Slice() result aliases source data")
				}
			}
		})
	}
}

func TestTensorSliceRejectsInvalidBounds(t *testing.T) {
	tensor := mustTensor(t, []float64{1, 2, 3, 4}, []int{2, 2})
	for _, bounds := range []struct{ starts, ends []int }{
		{[]int{0}, []int{1}},
		{[]int{-1, 0}, []int{1, 1}},
		{[]int{1, 0}, []int{0, 1}},
		{[]int{0, 0}, []int{3, 1}},
	} {
		if _, err := tensor.Slice(bounds.starts, bounds.ends); err == nil {
			t.Fatalf("Slice(%v, %v) accepted invalid bounds", bounds.starts, bounds.ends)
		}
	}
}

func mustTensor(t *testing.T, data []float64, shape []int) *Tensor {
	t.Helper()
	tensor, err := NewTensor(data, shape)
	if err != nil {
		t.Fatal(err)
	}
	return tensor
}

func TestTensorMatMul(t *testing.T) {
	left, err := NewTensor([]float64{1, 2, 3, 4, 5, 6}, []int{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewTensor([]float64{7, 8, 9, 10, 11, 12}, []int{3, 2})
	if err != nil {
		t.Fatal(err)
	}
	result, err := TensorMatMul(left, right)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{58, 64, 139, 154}
	for index := range want {
		if result.Data[index] != want[index] {
			t.Fatalf("result[%d]=%g, want %g", index, result.Data[index], want[index])
		}
	}
}

func TestTensorMatMulDType(t *testing.T) {
	left, err := NewTensorWithDType([]float64{1.1, 2.2}, []int{1, 2}, TensorFloat32)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewTensorWithDType([]float64{2, 3}, []int{2, 1}, TensorInt32)
	if err != nil {
		t.Fatal(err)
	}
	result, err := TensorMatMul(left, right)
	if err != nil {
		t.Fatal(err)
	}
	want := float64(float32(left.Data[0]*2 + left.Data[1]*3))
	if result.DType != TensorFloat32 || result.Data[0] != want {
		t.Fatalf("matmul=%v dtype=%q, want [%g] float32", result.Data, result.DType, want)
	}
}

func TestTensorMatMulParallelMatchesSerial(t *testing.T) {
	const rows, shared, columns = 83, 64, 53
	leftData := make([]float64, rows*shared)
	rightData := make([]float64, shared*columns)
	for index := range leftData {
		leftData[index] = float64(index%17-8) / 3
	}
	for index := range rightData {
		rightData[index] = float64(index%13-6) / 5
	}
	left := mustTensor(t, leftData, []int{rows, shared})
	right := mustTensor(t, rightData, []int{shared, columns})
	want := make([]float64, rows*columns)
	tensorMatMulRows(left, right, want, 0, rows)

	result, err := TensorMatMul(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Data, want) {
		t.Fatal("parallel TensorMatMul() differs from serial result")
	}
}

func TestTensorMatMulWorkerCount(t *testing.T) {
	tests := []struct {
		name                  string
		rows, columns, shared int
		wantSerial            bool
	}{
		{"small workload", 4, 4, 4, true},
		{"single row", 1, 1024, 1024, true},
		{"zero columns", 1000, 0, 1000, true},
		{"parallel workload", 83, 64, 53, runtime.GOMAXPROCS(0) == 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workers := tensorMatMulWorkerCount(test.rows, test.columns, test.shared)
			if (workers == 1) != test.wantSerial {
				t.Fatalf("worker count=%d, want serial=%t", workers, test.wantSerial)
			}
			if workers > test.rows {
				t.Fatalf("worker count=%d exceeds rows=%d", workers, test.rows)
			}
		})
	}
}

func BenchmarkTensorMatMul(b *testing.B) {
	for _, size := range []int{32, 128, 256} {
		left := benchmarkTensor(size, size)
		right := benchmarkTensor(size, size)
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			b.Run("serial", func(b *testing.B) {
				for b.Loop() {
					output := make([]float64, size*size)
					tensorMatMulRows(left, right, output, 0, size)
				}
			})
			b.Run("auto", func(b *testing.B) {
				for b.Loop() {
					if _, err := TensorMatMul(left, right); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func benchmarkTensor(rows, columns int) *Tensor {
	data := make([]float64, rows*columns)
	for index := range data {
		data[index] = float64(index%19-9) / 7
	}
	tensor, _ := NewTensor(data, []int{rows, columns})
	return tensor
}
