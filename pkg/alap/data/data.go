package data

import (
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Dataset represents a tabular data structure
type Dataset struct {
	Name    string                 `json:"name"`
	Columns []string               `json:"columns"`
	Rows    []map[string]float64   `json:"rows"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// NewDataset creates a new dataset
func NewDataset(name string, columns []string) *Dataset {
	return &Dataset{
		Name:    name,
		Columns: columns,
		Rows:    []map[string]float64{},
		Meta:    make(map[string]interface{}),
	}
}

// LoadCSV parses CSV data into a Dataset
func LoadCSV(name, csvContent string) (*Dataset, error) {
	reader := csv.NewReader(strings.NewReader(csvContent))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have header and at least 1 data row")
	}

	columns := records[0]
	ds := NewDataset(name, columns)

	for _, rec := range records[1:] {
		row := make(map[string]float64)
		for colIdx, colName := range columns {
			if colIdx < len(rec) {
				val, parseErr := strconv.ParseFloat(strings.TrimSpace(rec[colIdx]), 64)
				if parseErr == nil {
					row[colName] = val
				} else {
					row[colName] = math.NaN()
				}
			}
		}
		ds.Rows = append(ds.Rows, row)
	}

	return ds, nil
}

// AddRow appends a row
func (d *Dataset) AddRow(row map[string]float64) *Dataset {
	d.Rows = append(d.Rows, row)
	return d
}

// ─── TRANSFORMS ─────────────────────────────────────────────────────────────

type TransformFunc func(d *Dataset) *Dataset

// RemoveMissing removes any row containing NaN
func RemoveMissing(d *Dataset) *Dataset {
	cleaned := NewDataset(d.Name, d.Columns)
	for _, r := range d.Rows {
		hasNaN := false
		for _, v := range r {
			if math.IsNaN(v) {
				hasNaN = true
				break
			}
		}
		if !hasNaN {
			cleaned.Rows = append(cleaned.Rows, r)
		}
	}
	return cleaned
}

// Normalize rescales columns between 0 and 1
func Normalize(d *Dataset) *Dataset {
	if len(d.Rows) == 0 {
		return d
	}

	norm := NewDataset(d.Name, d.Columns)
	minVals := make(map[string]float64)
	maxVals := make(map[string]float64)

	for _, col := range d.Columns {
		minVals[col] = math.MaxFloat64
		maxVals[col] = -math.MaxFloat64
	}

	for _, r := range d.Rows {
		for _, col := range d.Columns {
			v := r[col]
			if !math.IsNaN(v) {
				if v < minVals[col] {
					minVals[col] = v
				}
				if v > maxVals[col] {
					maxVals[col] = v
				}
			}
		}
	}

	for _, r := range d.Rows {
		newRow := make(map[string]float64)
		for _, col := range d.Columns {
			v := r[col]
			span := maxVals[col] - minVals[col]
			if span == 0 || math.IsNaN(v) {
				newRow[col] = 0.0
			} else {
				newRow[col] = (v - minVals[col]) / span
			}
		}
		norm.Rows = append(norm.Rows, newRow)
	}

	return norm
}

// ─── MODEL: LINEAR REGRESSION ───────────────────────────────────────────────

type LinearRegression struct {
	Target   string             `json:"target"`
	Features []string           `json:"features"`
	Weights  map[string]float64 `json:"weights"`
	Bias     float64            `json:"bias"`
	Trained  bool               `json:"trained"`
}

func NewLinearRegression(target string, features []string) *LinearRegression {
	return &LinearRegression{
		Target:   target,
		Features: features,
		Weights:  make(map[string]float64),
	}
}

// Train trains weights using batch gradient descent
func (m *LinearRegression) Train(d *Dataset, epochs int, lr float64) error {
	if len(d.Rows) == 0 {
		return fmt.Errorf("cannot train on empty dataset")
	}

	// Initialize weights
	for _, f := range m.Features {
		m.Weights[f] = 0.0
	}
	m.Bias = 0.0

	n := float64(len(d.Rows))

	for epoch := 0; epoch < epochs; epoch++ {
		weightGradients := make(map[string]float64)
		var biasGradient float64

		for _, r := range d.Rows {
			actual := r[m.Target]
			predicted := m.Predict(r)
			errorDiff := predicted - actual

			for _, f := range m.Features {
				weightGradients[f] += (2.0 / n) * errorDiff * r[f]
			}
			biasGradient += (2.0 / n) * errorDiff
		}

		// Update weights
		for _, f := range m.Features {
			m.Weights[f] -= lr * weightGradients[f]
		}
		m.Bias -= lr * biasGradient
	}

	m.Trained = true
	return nil
}

// Predict calculates target estimate from feature values
func (m *LinearRegression) Predict(features map[string]float64) float64 {
	pred := m.Bias
	for _, f := range m.Features {
		pred += m.Weights[f] * features[f]
	}
	return pred
}

// ─── EVALUATION METRICS ─────────────────────────────────────────────────────

// MSE calculates Mean Squared Error
func MSE(actual, predicted []float64) float64 {
	if len(actual) == 0 || len(actual) != len(predicted) {
		return 0.0
	}
	var sum float64
	for i := 0; i < len(actual); i++ {
		diff := actual[i] - predicted[i]
		sum += diff * diff
	}
	return sum / float64(len(actual))
}

// R2Score calculates Coefficient of Determination (R^2)
func R2Score(actual, predicted []float64) float64 {
	if len(actual) == 0 || len(actual) != len(predicted) {
		return 0.0
	}
	var meanActual float64
	for _, a := range actual {
		meanActual += a
	}
	meanActual /= float64(len(actual))

	var ssTot, ssRes float64
	for i := 0; i < len(actual); i++ {
		ssTot += math.Pow(actual[i]-meanActual, 2)
		ssRes += math.Pow(actual[i]-predicted[i], 2)
	}

	if ssTot == 0 {
		return 1.0
	}
	return 1.0 - (ssRes / ssTot)
}
