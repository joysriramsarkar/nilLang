package textbn

import (
	"strings"
	"testing"
	"time"
)

func TestDigits(t *testing.T) {
	if ToBengali("0123456789") != "০১২৩৪৫৬৭৮৯" {
		t.Fatal("digit conversion broken")
	}
	if ToASCII("১২৩") != "123" {
		t.Fatal("reverse conversion broken")
	}
	if ToBengali("৳1,250.00") != "৳১,২৫০.০০" {
		t.Fatal("mixed broken")
	}
}

func TestGrouping(t *testing.T) {
	cases := map[int64]string{456: "456", 1234: "1,234", 123456: "1,23,456", 1234567: "12,34,567", 12345678: "1,23,45,678"}
	for n, want := range cases {
		if got := GroupBD(n); got != want {
			t.Errorf("GroupBD(%d)=%q want %q", n, got, want)
		}
	}
	if GroupBengali(123456) != "১,২৩,৪৫৬" {
		t.Fatal("bengali grouping broken")
	}
	if LakhCrore(500000) != "5 লক্ষ" || LakhCrore(20000000) != "2 কোটি" {
		t.Fatalf("lakh/crore broken: %q %q", LakhCrore(500000), LakhCrore(20000000))
	}
}

func TestBangabda(t *testing.T) {
	// Pohela Boishakh 1432 ≈ April 14 2025.
	y, m, d := BangabdaDate(time.Date(2025, 4, 14, 0, 0, 0, 0, time.UTC))
	if y != 1432 || m != "বৈশাখ" || d != 1 {
		t.Fatalf("boishakh broken: %d %s %d", y, m, d)
	}
	y2, _, _ := BangabdaDate(time.Date(2025, 4, 13, 0, 0, 0, 0, time.UTC))
	if y2 != 1431 {
		t.Fatalf("eve must be 1431, got %d", y2)
	}
}

func TestClustersJuktakkhar(t *testing.T) {
	// ক্ষ = ক + ্ + ষ stays one cluster.
	if cls := Clusters("ক্ষ"); len(cls) != 1 {
		t.Fatalf("ক্ষ must be 1 cluster: %q", cls)
	}
	// স্ত্র = স + ্ + ত + ্ + র one cluster.
	if cls := Clusters("স্ত্র"); len(cls) != 1 {
		t.Fatalf("স্ত্র must be 1 cluster: %q", cls)
	}
	// কি = ক + ি one cluster.
	if cls := Clusters("কি"); len(cls) != 1 {
		t.Fatalf("কি must be 1 cluster: %q", cls)
	}
	// ক + ি + separate ক = 2 clusters.
	if cls := Clusters("কিক"); len(cls) != 2 {
		t.Fatalf("কিক must be 2 clusters: %q", cls)
	}
	// Latin combining mark stays.
	if cls := Clusters("e\u0301x"); len(cls) != 2 {
		t.Fatalf("combining broken: %q", cls)
	}
}

func TestShaping(t *testing.T) {
	sh := Shaped("কি")
	if len(sh) != 1 {
		t.Fatalf("shaped len: %q", sh)
	}
	if rs := []rune(sh[0]); rs[0] != 0x09BF {
		t.Fatalf("pre-base ি must lead display order: %U", rs)
	}
	if !HasReph("র্" + "ক") {
		t.Fatal("reph not detected")
	}
	if HasReph("কি") {
		t.Fatal("false reph")
	}
}

func TestBreakLines(t *testing.T) {
	lines := BreakLines("আমি বাংলায় গান গাই", 8)
	if len(lines) < 2 {
		t.Fatalf("must wrap: %q", lines)
	}
	for _, l := range lines[1:] {
		if l == "" {
			t.Fatal("empty wrapped line")
		}
	}
}

func TestNFCAndWellFormed(t *testing.T) {
	if !IsWellFormed("বাংলা") || IsWellFormed("্ক") {
		t.Fatal("wellformed broken")
	}
	if NormalizeNFC("বাংলা") != "বাংলা" {
		t.Fatal("nfc broken")
	}
}

func TestFallback(t *testing.T) {
	stack := []string{"Roboto", "Noto Sans Bengali"}
	if Fallback('ক', stack) != "Noto Sans Bengali" {
		t.Fatal("bengali fallback broken")
	}
	if Fallback('A', stack) != "Roboto" {
		t.Fatal("latin fallback broken")
	}
}

func TestEllipsisLineHeight(t *testing.T) {
	e := Ellipsize("বাংলাদেশ", 6)
	if !strings.HasSuffix(e, "…") {
		t.Fatalf("ellipsis broken: %q", e)
	}
	if LineHeight(16, 1.2) != 24 {
		t.Fatalf("min line-height 1.5 broken: %v", LineHeight(16, 1.2))
	}
}

func TestSelection(t *testing.T) {
	s := "কিখ"
	sel := SelectClusters(99, 2, 0) // clamps + orders
	if sel.Start != 0 {
		t.Fatalf("clamp broken: %+v", sel)
	}
	if got := SelectedText(s, Selection{0, 1}); got != "কি" {
		t.Fatalf("selection split cluster: %q", got)
	}
}

func TestFormatDateBN(t *testing.T) {
	d := FormatDateBN(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(d, "অক্টোবর") || !strings.Contains(d, "১২") || !strings.Contains(d, "২০২৬") {
		t.Fatalf("bn date broken: %q", d)
	}
}

// Golden volume: cluster segmentation across conjuncts (font-independent).
func TestConjunctGoldens(t *testing.T) {
	cases := map[string]int{
		"ক্ষ": 1, "জ্ঞ": 1, "ত্র": 1, "শ্র": 1, "ক্র": 1, "গ্র": 1,
		"ঙ্ক": 1, "ণ্ড": 1, "ন্ত": 1, "ম্প": 1, "স্ক": 1, "স্প": 1,
		"রং": 1, "কিং": 1, "বাংলা": 2, "যুক্তাক্ষর": 4,
		"কর্ম": 2, "ধর্ম": 2, "সূর্য": 2, "পূর্ব": 2,
		"বিজ্ঞান": 3, "স্বাগতম": 4, "বিদ্যালয়": 4, "চট্টগ্রাম": 4,
		"ক্রিকেট": 3, "ফুটবল": 4, "স্টেডিয়াম": 4,
		"রেফ": 2, "কর্ণ": 2, "অর্ঘ্য": 2,
		"০১২৩৪৫৬৭৮৯": 10, "১,২৩,৪৫৬": 8,
		"hello": 5, "কিhello": 6,
	}
	for in, want := range cases {
		if got := len(Clusters(in)); got != want {
			t.Errorf("Clusters(%q)=%d want %d (%q)", in, got, want, Clusters(in))
		}
		// Shaping preserves cluster count.
		if got := len(Shaped(in)); got != want {
			t.Errorf("Shaped(%q)=%d want %d", in, got, want)
		}
	}
}
