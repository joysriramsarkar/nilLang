package data

import (
	"math"
	"testing"
)

func TestAlapDataPipeline(t *testing.T) {
	// Sample sales CSV matching Section 18 of refactor.md
	csvData := `price,quantity,revenue
10,2,20
20,1,20
30,3,90
40,2,80
`
	ds, err := LoadCSV("Sales", csvData)
	if err != nil {
		t.Fatalf("LoadCSV failed: %v", err)
	}

	if len(ds.Rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(ds.Rows))
	}

	// Test Transform: Normalize
	normalized := Normalize(ds)
	if len(normalized.Rows) != 4 {
		t.Fatalf("expected 4 rows after normalize")
	}

	// Test Model: LinearRegression
	model := NewLinearRegression("revenue", []string{"price", "quantity"})
	err = model.Train(ds, 500, 0.0001)
	if err != nil {
		t.Fatalf("training failed: %v", err)
	}

	pred := model.Predict(map[string]float64{"price": 10, "quantity": 2})
	if math.IsNaN(pred) {
		t.Errorf("prediction returned NaN")
	}

	// Test Evaluation Metrics
	actuals := []float64{20, 20, 90, 80}
	predictions := []float64{22, 19, 88, 81}
	mse := MSE(actuals, predictions)
	if mse > 5.0 {
		t.Errorf("expected low MSE, got %f", mse)
	}

	r2 := R2Score(actuals, predictions)
	if r2 < 0.95 {
		t.Errorf("expected high R^2, got %f", r2)
	}
}
