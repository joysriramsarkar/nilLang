package i18n

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// Supported locales
const (
	LocaleBnBD = "bn-BD"
	LocaleBnIN = "bn-IN"
	LocaleEnIN = "en-IN"
	LocaleEnUS = "en-US"
)

var (
	bnDigits = [10]rune{'০', '১', '২', '৩', '৪', '৫', '৬', '৭', '৮', '৯'}
	enDigits = [10]rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

	bnMonths = [12]string{
		"জানুয়ারি", "ফেব্রুয়ারি", "মার্চ", "এপ্রিল", "মে", "জুন",
		"জুলাই", "আগস্ট", "সেপ্টেম্বর", "অক্টোবর", "নভেম্বর", "ডিসেম্বর",
	}

	bnDays = [7]string{
		"রবিবার", "সোমবার", "মঙ্গলবার", "বুধবার", "বৃহস্পতিবার", "শুক্রবার", "শনিবার",
	}
)

// ToBengaliDigits converts ASCII digits to Bengali digits (e.g. "1250" -> "১২৫০")
func ToBengaliDigits(s string) string {
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

// FromBengaliDigits converts Bengali digits to ASCII digits (e.g. "১২৫০" -> "1250")
func FromBengaliDigits(s string) string {
	var sb strings.Builder
	for _, r := range s {
		found := false
		for i, bn := range bnDigits {
			if r == bn {
				sb.WriteRune(enDigits[i])
				found = true
				break
			}
		}
		if !found {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// FormatMoney formats money in given locale with currency symbol and appropriate numeral system
func FormatMoney(m data.Money, locale string) string {
	formatted := m.Format() // e.g. "৳1,250.00"
	if strings.HasPrefix(locale, "bn") {
		return ToBengaliDigits(formatted) // e.g. "৳১,২৫০.০০"
	}
	return formatted
}

// FormatDate formats time in localized representation
func FormatDate(t time.Time, locale string) string {
	if strings.HasPrefix(locale, "bn") {
		day := ToBengaliDigits(fmt.Sprintf("%02d", t.Day()))
		month := bnMonths[t.Month()-1]
		year := ToBengaliDigits(fmt.Sprintf("%04d", t.Year()))
		return fmt.Sprintf("%s %s %s", day, month, year)
	}
	return t.Format("02 Jan 2006")
}

// FormatDateTime formats date and time
func FormatDateTime(t time.Time, locale string) string {
	d := FormatDate(t, locale)
	timeStr := t.Format("03:04:05 PM")
	if strings.HasPrefix(locale, "bn") {
		timeStr = ToBengaliDigits(timeStr)
	}
	return fmt.Sprintf("%s, %s", d, timeStr)
}

// FormatDayOfWeek returns the localized name of the weekday
func FormatDayOfWeek(t time.Time, locale string) string {
	if strings.HasPrefix(locale, "bn") {
		return bnDays[t.Weekday()]
	}
	return t.Weekday().String()
}

// Catalog holds i18n dictionaries
type Catalog struct {
	mu           sync.RWMutex
	translations map[string]map[string]string // locale -> key -> value
}

var globalCatalog = &Catalog{
	translations: make(map[string]map[string]string),
}

func init() {
	// Seed baseline translations
	RegisterTranslations(LocaleBnBD, map[string]string{
		"pos.title":          "লাখান ভাণ্ডার পিওএস",
		"cart.title":         "চলতি কার্ট",
		"cart.empty":         "কার্ট বর্তমানে খালি",
		"cart.subtotal":      "মোট মূল্য",
		"cart.discount":      "ছাড়",
		"cart.tax":           "ভ্যাট / কর",
		"cart.grand_total":   "সর্বমোট দেয়",
		"checkout.pay":       "মূল্য পরিশোধ",
		"checkout.cash":      "নগদ প্রদান",
		"checkout.change":    "ফেরত টাকা",
		"search.placeholder": "পণ্য বা বারকোড খুঁজুন...",
		"customer.due":       "বাকির পরিমাণ",
		"receipt.thank_you":  "ধন্যবাদ, আবার আসবেন!",
		"status.online":      "অনলাইন",
		"status.offline":     "অফলাইন (সিঙ্ক অপেক্ষারত)",
	})

	RegisterTranslations(LocaleEnIN, map[string]string{
		"pos.title":          "Lakhan Bhandar POS",
		"cart.title":         "Active Cart",
		"cart.empty":         "Cart is currently empty",
		"cart.subtotal":      "Subtotal",
		"cart.discount":      "Discount",
		"cart.tax":           "Tax / VAT",
		"cart.grand_total":   "Grand Total",
		"checkout.pay":       "Checkout",
		"checkout.cash":      "Cash Received",
		"checkout.change":    "Change Due",
		"search.placeholder": "Search product or scan barcode...",
		"customer.due":       "Due Balance",
		"receipt.thank_you":  "Thank you, visit again!",
		"status.online":      "Online",
		"status.offline":     "Offline (Sync Queued)",
	})
}

// RegisterTranslations adds translations for a locale
func RegisterTranslations(locale string, dict map[string]string) {
	globalCatalog.mu.Lock()
	defer globalCatalog.mu.Unlock()
	if _, ok := globalCatalog.translations[locale]; !ok {
		globalCatalog.translations[locale] = make(map[string]string)
	}
	for k, v := range dict {
		globalCatalog.translations[locale][k] = v
	}
}

// T translates a key into requested locale with fallback to default
func T(locale, key string) string {
	globalCatalog.mu.RLock()
	defer globalCatalog.mu.RUnlock()

	if dict, ok := globalCatalog.translations[locale]; ok {
		if val, found := dict[key]; found {
			return val
		}
	}

	// Fallback to bn-BD
	if dict, ok := globalCatalog.translations[LocaleBnBD]; ok {
		if val, found := dict[key]; found {
			return val
		}
	}

	return key
}
