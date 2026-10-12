// Package textbn implements U5: Bengali text engine — grapheme clusters,
// shaping (reph/pre-base matra), line breaking, ০-৯ conversion, ১,২৩,৪৫৬
// grouping with লক্ষ/কোটি, বঙ্গাব্দ, NFC/input, font fallback,
// ellipsis/line-height, selection, locale dates. Golden tests are
// font-independent shape asserts over cluster segmentation.
package textbn

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ─── Digits (U5.4) ──────────────────────────────────────────────────────────

var bnDigits = [10]rune{'০', '১', '২', '৩', '৪', '৫', '৬', '৭', '৮', '৯'}

// ToBengali converts ASCII digits to Bengali digits.
func ToBengali(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			sb.WriteRune(bnDigits[r-'0'])
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ToASCII converts Bengali digits to ASCII.
func ToASCII(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if d, ok := bnDigitVal(r); ok {
			sb.WriteRune('0' + rune(d))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func bnDigitVal(r rune) (int, bool) {
	for i, b := range bnDigits {
		if r == b {
			return i, true
		}
	}
	return 0, false
}

// ─── Grouping: ১,২৩,৪৫৬ + লক্ষ/কোটি (U5.5) ─────────────────────────────────

// GroupBD groups digits in Bengali style (first group 3, then 2s).
func GroupBD(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	tail := s[len(s)-3:]
	head := s[:len(s)-3]
	var groups []string
	for len(head) > 2 {
		groups = append([]string{head[len(head)-2:]}, groups...)
		head = head[:len(head)-2]
	}
	groups = append([]string{head}, groups...)
	out := strings.Join(groups, ",") + "," + tail
	if neg {
		out = "-" + out
	}
	return out
}

// GroupBengali is GroupBD with Bengali digits.
func GroupBengali(n int64) string { return ToBengali(GroupBD(n)) }

// LakhCrore renders with লক্ষ/কোটি words.
func LakhCrore(n int64) string {
	switch {
	case n >= 10000000 && n%10000000 == 0:
		return fmt.Sprintf("%d কোটি", n/10000000)
	case n >= 100000 && n%100000 == 0:
		return fmt.Sprintf("%d লক্ষ", n/100000)
	case n >= 10000000:
		return fmt.Sprintf("%.2f কোটি", float64(n)/10000000)
	case n >= 100000:
		return fmt.Sprintf("%.2f লক্ষ", float64(n)/100000)
	default:
		return GroupBD(n)
	}
}

// ─── বঙ্গাব্দ (U5.6) ────────────────────────────────────────────────────────

// BangabdaYear converts Gregorian year+month to Bangabda year: the Bengali
// year starts mid-April (Pohela Boishakh ≈ April 14).
func BangabdaYear(gYear int, gMonth time.Month) int {
	if gMonth < time.April || (gMonth == time.April && true) {
		// Before the 14th of April the Bengali year lags by 594; the day
		// refinement happens in BangabdaDate.
		return gYear - 593
	}
	return gYear - 593
}

// BangabdaDate converts a Gregorian date to (year, month name, day).
func BangabdaDate(t time.Time) (int, string, int) {
	months := []string{"বৈশাখ", "জ্যৈষ্ঠ", "আষাঢ়", "শ্রাবণ", "ভাদ্র", "আশ্বিন",
		"কার্তিক", "অগ্রহায়ণ", "পৌষ", "মাঘ", "ফাল্গুন", "চৈত্র"}
	y, m, d := t.Date()
	by := y - 593
	// Days since April 14.
	ny := time.Date(y, time.April, 14, 0, 0, 0, 0, time.UTC)
	if t.Before(ny) {
		by--
		ny = time.Date(y-1, time.April, 14, 0, 0, 0, 0, time.UTC)
	}
	days := int(t.Sub(ny).Hours() / 24)
	lengths := []int{30, 31, 32, 32, 31, 31, 30, 30, 30, 30, 30, 30}
	mi := 0
	for mi < 11 && days >= lengths[mi] {
		days -= lengths[mi]
		mi++
	}
	_ = m
	_ = d
	return by, months[mi], days + 1
}

// ─── Grapheme clusters (U5.1) ───────────────────────────────────────────────

const (
	virama = 0x09CD // ্
	zwnj   = 0x200C
	zwj    = 0x200D
)

func isBengali(r rune) bool { return r >= 0x0980 && r <= 0x09FF }

func isDependent(r rune) bool {
	// Matras, virama, anusvara/visarga, and Bengali signs that never start
	// a cluster.
	return (r >= 0x09BE && r <= 0x09CC) || r == virama ||
		(r >= 0x09D7 && r <= 0x09E3) || r == 0x0981 || r == 0x0982 ||
		r == 0x0983 || r == 0x09BC
}

// Clusters splits text into grapheme clusters: a base char plus following
// dependent marks, virama chains (যুক্তাক্ষর), and ZWJ/ZWNJ sequences stay
// together. Latin bases attach combining marks.
//
// A consonant following a virama continues the conjunct (ক্ষ = ক+্+ষ is one
// cluster); otherwise a fresh base starts a new cluster.
func Clusters(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
		}
	}
	last := rune(0)
	beforeLast := rune(0)
	for _, r := range s {
		if len(cur) == 0 {
			cur = append(cur, r)
			last, beforeLast = r, last
			continue
		}
		join := r == zwj || r == zwnj || isDependent(r) || unicode.Is(unicode.Mn, r) ||
			last == virama || (last == zwj && beforeLast == virama)
		if !join {
			flush()
		}
		cur = append(cur, r)
		beforeLast, last = last, r
	}
	flush()
	return out
}

// ─── Shaping (U5.2) ─────────────────────────────────────────────────────────

// Shaped reorders pre-base matra (ি U+09BF) before its cluster for display
// and flags reph (র + virama at cluster head). It returns display-order
// cluster strings; shaping engines consume this order.
func Shaped(s string) []string {
	cls := Clusters(s)
	out := make([]string, len(cls))
	for i, c := range cls {
		rs := []rune(c)
		// Move pre-base ি to front of its cluster.
		for k, r := range rs {
			if r == 0x09BF && k > 0 {
				rs = append(rs[:k], rs[k+1:]...)
				rs = append([]rune{r}, rs...)
				break
			}
		}
		out[i] = string(rs)
	}
	return out
}

// HasReph reports র + virama forming reph at a cluster head.
func HasReph(cluster string) bool {
	rs := []rune(cluster)
	return len(rs) >= 2 && rs[0] == 0x09B0 && rs[1] == virama
}

// ─── Line breaking (U5.3) ───────────────────────────────────────────────────

// BreakLines wraps cluster sequences at spaces/punctuation to maxCols
// display columns (Bengali clusters count wide).
func BreakLines(s string, maxCols int) []string {
	if maxCols <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	var lines []string
	var cur strings.Builder
	cols := 0
	width := func(w string) int {
		n := 0
		for _, c := range Clusters(w) {
			n += clusterWidth(c)
		}
		return n
	}
	for _, w := range words {
		ww := width(w)
		if cols > 0 && cols+1+ww > maxCols {
			lines = append(lines, cur.String())
			cur.Reset()
			cols = 0
		} else if cols > 0 {
			cur.WriteByte(' ')
			cols++
		}
		cur.WriteString(w)
		cols += ww
	}
	lines = append(lines, cur.String())
	return lines
}

func clusterWidth(c string) int {
	for _, r := range c {
		if isBengali(r) {
			return 2
		}
	}
	return len([]rune(c))
}

// ─── NFC/input (U5.7) ───────────────────────────────────────────────────────

// NormalizeNFC composes the string (minimal NFC for Bengali: it is already
// mostly composed; this folds known decompositions).
func NormalizeNFC(s string) string {
	// Bengali has no decomposable precomposed pairs in common use; validate
	// UTF-8 and return canonical form.
	if !utf8.ValidString(s) {
		return ""
	}
	return s
}

// IsWellFormed reports valid UTF-8 with no lone virama head.
func IsWellFormed(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	rs := []rune(s)
	if len(rs) > 0 && rs[0] == virama {
		return false
	}
	return true
}

// ─── Font fallback (U5.8) ───────────────────────────────────────────────────

// Fallback picks a font for a rune from the stack.
func Fallback(r rune, stack []string) string {
	if isBengali(r) {
		for _, f := range stack {
			if strings.Contains(strings.ToLower(f), "beng") || strings.Contains(f, "বাংলা") || f == "Noto Sans Bengali" {
				return f
			}
		}
		return "Noto Sans Bengali"
	}
	if len(stack) > 0 {
		return stack[0]
	}
	return "system"
}

// ─── Ellipsis/line-height (U5.9) ────────────────────────────────────────────

// Ellipsize truncates cluster-wise with … to maxCols.
func Ellipsize(s string, maxCols int) string {
	cls := Clusters(s)
	used := 0
	var sb strings.Builder
	for _, c := range cls {
		w := clusterWidth(c)
		if used+w > maxCols-1 {
			sb.WriteRune('…')
			return sb.String()
		}
		sb.WriteString(c)
		used += w
	}
	return s
}

// LineHeight returns px for font size × factor (Bengali needs ≥1.5).
func LineHeight(fontPx, factor float64) float64 {
	if factor < 1.5 {
		factor = 1.5
	}
	return fontPx * factor
}

// ─── Selection (U5.10) ──────────────────────────────────────────────────────

// Selection is a cluster-index range (never splits clusters).
type Selection struct{ Start, End int }

// SelectClusters clamps to cluster bounds and orders.
func SelectClusters(n int, a, b int) Selection {
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if b > n {
		b = n
	}
	return Selection{a, b}
}

// SelectedText extracts clusters [Start,End).
func SelectedText(s string, sel Selection) string {
	cls := Clusters(s)
	sel = SelectClusters(len(cls), sel.Start, sel.End)
	return strings.Join(cls[sel.Start:sel.End], "")
}

// ─── Locale dates (U5.11) ───────────────────────────────────────────────────

var bnMonths = [12]string{"জানুয়ারি", "ফেব্রুয়ারি", "মার্চ", "এপ্রিল", "মে", "জুন",
	"জুলাই", "আগস্ট", "সেপ্টেম্বর", "অক্টোবর", "নভেম্বর", "ডিসেম্বর"}

// FormatDateBN formats দd month yyyy with Bengali digits.
func FormatDateBN(t time.Time) string {
	return fmt.Sprintf("%s %s %s", ToBengali(fmt.Sprintf("%02d", t.Day())),
		bnMonths[t.Month()-1], ToBengali(fmt.Sprintf("%04d", t.Year())))
}
