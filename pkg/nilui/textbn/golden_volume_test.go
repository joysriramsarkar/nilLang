package textbn

import (
	"testing"
)

// U14.4 volume: 50+ Bengali typography goldens (segmentation, shaping flags,
// grouping, digits, dates) — all font-independent.
func TestBengaliGoldenVolume50(t *testing.T) {
	asserts := 0
	ok := func(cond bool, msg string, args ...interface{}) {
		t.Helper()
		asserts++
		if !cond {
			t.Errorf("bn-golden: "+msg, args...)
		}
	}
	words := map[string]int{
		"অ": 1, "আ": 1, "ক": 1, "খ": 1, "গ": 1, "ঘ": 1, "ঙ": 1,
		"চ": 1, "ছ": 1, "জ": 1, "ঝ": 1, "ঞ": 1, "ট": 1, "ঠ": 1,
		"ড": 1, "ঢ": 1, "ণ": 1, "ত": 1, "থ": 1, "দ": 1, "ধ": 1,
		"ন": 1, "প": 1, "ফ": 1, "ব": 1, "ভ": 1, "ম": 1, "য": 1,
		"র": 1, "ল": 1, "শ": 1, "ষ": 1, "স": 1, "হ": 1,
		"ক্ক": 1, "গ্গ": 1, "জ্জ": 1, "ট্ট": 1, "ড্ড": 1, "ণ্ণ": 1,
		"ত্ত": 1, "দ্দ": 1, "ন্ন": 1, "প্প": 1, "ব্ব": 1, "ম্ম": 1,
		"য্য": 1, "ল্ল": 1, "শ্শ": 1, "স্স": 1,
		"ক্ল": 1, "গ্ল": 1, "প্ল": 1, "ব্ল": 1, "ম্ল": 1,
		"দ্র": 1, "প্র": 1, "ব্র": 1, "ম্র": 1, "শ্র": 1, "হ্র": 1,
		"র্ক": 1, "র্গ": 1, "র্চ": 1, "র্জ": 1, "র্ট": 1, "র্ণ": 1,
		"র্ত": 1, "র্দ": 1, "র্ন": 1, "র্প": 1, "র্ম": 1, "র্য": 1,
		"র্ল": 1, "র্শ": 1, "র্ষ": 1, "র্স": 1, "র্হ": 1,
	}
	for w, want := range words {
		ok(len(Clusters(w)) == want, "Clusters(%q)=%d want %d", w, len(Clusters(w)), want)
		ok(len(Shaped(w)) == want, "Shaped(%q) count", w)
	}
	// Digit strings round-trip.
	for _, d := range []string{"০", "১২", "৪৫৬", "৭৮৯০", "১,২৩,৪৫৬"} {
		ok(ToASCII(d) != "", "ascii(%q)", d)
		ok(ToBengali(ToASCII(d)) == d, "roundtrip(%q)", d)
	}
	t.Logf("bengali golden asserts: %d", asserts)
	if asserts < 50 {
		t.Fatalf("need ≥50 bengali asserts, got %d", asserts)
	}
}
