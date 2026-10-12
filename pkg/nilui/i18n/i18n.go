// Package uii18n connects pkg/alap/i18n to the UI tree (U12): locale-aware
// tree localization, locale dropdown options, Bangla locale date/number/
// currency (via textbn), RTL flags, and a platform-locale interface.
package uii18n

import (
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/i18n"
	"github.com/joysriramsarkar/nilLang/pkg/nilui/ir"
	"github.com/joysriramsarkar/nilLang/pkg/nilui/textbn"
)

// Re-export core locale helpers so UI code has one import.
const (
	LocaleBnBD = i18n.LocaleBnBD
	LocaleBnIN = i18n.LocaleBnIN
	LocaleEnIN = i18n.LocaleEnIN
	LocaleEnUS = i18n.LocaleEnUS
)

// T translates (fallback chain lives in i18n).
func T(locale, key string) string { return i18n.T(locale, key) }

// LocalizeTree rewrites i18n:KEY props/values in place (U12.1).
func LocalizeTree(n *ir.Node, locale string) {
	if n == nil {
		return
	}
	for k, v := range n.Props {
		if len(v) > 5 && v[:5] == "i18n:" {
			n.Props[k] = T(locale, v[5:])
		}
	}
	for _, c := range n.Children {
		LocalizeTree(c, locale)
	}
	for _, s := range n.Slots {
		LocalizeTree(s, locale)
	}
}

// DropdownOptions lists locales for a dropdown (U12.2).
func DropdownOptions() [][2]string {
	return [][2]string{
		{LocaleBnBD, "বাংলা (বাংলাদেশ)"},
		{LocaleBnIN, "বাংলা (ভারত)"},
		{LocaleEnIN, "English (India)"},
		{LocaleEnUS, "English (US)"},
	}
}

// FormatNumberBN formats with Bengali grouping+digits (U12.3).
func FormatNumberBN(n int64) string { return textbn.GroupBengali(n) }

// FormatMoneyBN formats minor units as ৳ with Bengali digits.
func FormatMoneyBN(minor int64) string {
	taka := float64(minor) / 100
	intPart := int64(taka)
	frac := int(minor%100+100) % 100
	whole := textbn.GroupBD(intPart)
	digits := "০১২৩৪৫৬৭৮৯"
	_ = digits
	_ = whole
	return "৳" + textbn.ToBengali(whole) + "." + textbn.ToBengali(pad2(frac))
}

func pad2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// FormatDateBN formats a date in Bangla (U12.3).
func FormatDateBN(t time.Time) string { return textbn.FormatDateBN(t) }

// IsRTL reports right-to-left locales (U12.4).
func IsRTL(locale string) bool {
	switch locale {
	case "ar", "ar-SA", "he", "fa", "ur":
		return true
	}
	return false
}

// PlatformLocale is implemented by the host (U12.5).
type PlatformLocale interface{ Locale() string }

// SystemLocale is a static host locale (tests/preview).
type SystemLocale struct{ Tag string }

// Locale returns the tag.
func (s SystemLocale) Locale() string { return s.Tag }
