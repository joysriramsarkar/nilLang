package i18n

import (
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

func TestBengaliDigitsConversion(t *testing.T) {
	ascii := "1250.75"
	bn := ToBengaliDigits(ascii)
	expected := "১২৫০.৭৫"
	if bn != expected {
		t.Fatalf("Expected %s, got %s", expected, bn)
	}

	convertedBack := FromBengaliDigits(bn)
	if convertedBack != ascii {
		t.Fatalf("Expected %s, got %s", ascii, convertedBack)
	}
}

func TestBengaliMoneyAndDateFormatting(t *testing.T) {
	m := data.NewMoney(340000, "BDT") // ৳3,400.00
	bnFormatted := FormatMoney(m, LocaleBnBD)
	expectedMoney := "৳৩,৪০০.০০"
	if bnFormatted != expectedMoney {
		t.Fatalf("Expected %s, got %s", expectedMoney, bnFormatted)
	}

	testDate := time.Date(2026, time.September, 7, 18, 30, 0, 0, time.UTC)
	dStr := FormatDate(testDate, LocaleBnBD)
	if dStr != "০৭ সেপ্টেম্বর ২০২৬" {
		t.Fatalf("Expected '০৭ সেপ্টেম্বর ২০২৬', got '%s'", dStr)
	}

	// Translation
	title := T(LocaleBnBD, "pos.title")
	if title != "লাখান ভাণ্ডার পিওএস" {
		t.Fatalf("Expected 'লাখান ভাণ্ডার পিওএস', got '%s'", title)
	}
}
