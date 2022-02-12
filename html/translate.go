package html

import (
	"net/http"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// https://ec.europa.eu/eurostat/statistics-explained/index.php?title=Glossary:Country_codes/de
var euCountryCodes = [...]string{"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI", "FR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK"}

type TagStr struct {
	Tag language.Tag
	Str string
}

type IDName struct {
	ID   string
	Name string // sort by name
}

// global variable used by type Language
var translations = map[string][]TagStr{
	"how": []TagStr{
		TagStr{language.AmericanEnglish, "How does it work?"},
		TagStr{language.German, "Wie funktioniert die Bestellung?"},
	},
	"how-1": []TagStr{
		TagStr{language.AmericanEnglish, "Enter the amount and press „Order“."},
		TagStr{language.German, "Wähle die gewünschte Anzahl aus und klicke „Bestellen“."},
	},
	"how-2": []TagStr{
		TagStr{language.AmericanEnglish, "Save the URL of your order. You need it to access your goods if you closed the browser tab."},
		TagStr{language.German, "Speichere die Webadresse deiner Bestellung. Du brauchst sie, um auf deine Güter zuzugreifen, falls du das Browserfenster geschlossen hast."},
	},
	"how-3": []TagStr{
		TagStr{language.AmericanEnglish, "Pay your order using one of these methods. (Unpaid orders are deleted after 30 days.)"},
		TagStr{language.German, "Bezahle deine Bestellung mit einer dieser Methoden. (Unbezahlte Bestellungen werden nach 30 Tagen gelöscht.)"},
	},
	"how-3a": []TagStr{
		TagStr{language.AmericanEnglish, "Monero (XMR) or Bitcoin (BTC): The fastest method. Your voucher codes are shown as soon as your payment is confirmed on the blockchain."},
		TagStr{language.German, "Monero (XMR) oder Bitcoin (BTC): Die schnellste Methode. Deine Gutscheincodes werden angezeigt, sobald deine Zahlung in der Blockchain bestätigt ist."},
	},
	"how-3b": []TagStr{
		TagStr{language.AmericanEnglish, "Cash: Send an insured letter or package with cash to our office in Germany. We shred the letter after processing."},
		TagStr{language.German, "Bargeld: Schicke Bargeld in einem versichertem Brief oder Paket an unsere Adresse in Deutschland. Wir schreddern den Brief nach dem Freischalten."},
	},
	"how-3c": []TagStr{
		TagStr{language.AmericanEnglish, "SEPA (Single Euro Payments Area) bank transfer to our German bank account. We manually check for new incoming payments every day."},
		TagStr{language.German, "SEPA-Überweisung auf unser deutsches Bankkonto. Wir prüfen es täglich manuell auf neue Zahlungseingänge."},
	},
	"how-4": []TagStr{
		TagStr{language.AmericanEnglish, "As soon as your payment arrives, your voucher codes are shown. In the unlikely case that your goods have become sold out in the meantime, your codes will appear as soon as they are back in stock."},
		TagStr{language.German, "Sobald deine Zahlung bei uns eintrifft, werden dir deine Gutscheincodes angezeigt. In seltenen Fällen kann es passieren, dass das Produkt zwischenzeitlich ausverkauft ist. Dann werden dir die Codes angezeigt, sobald Nachschub da ist."},
	},
	"how-5": []TagStr{
		TagStr{language.AmericanEnglish, "Write down your codes. We will delete them 30 days after delivery."},
		TagStr{language.German, "Notiere dir die Codes. Wir werden sie 30 Tage nach der Auslieferung löschen."},
	},
	"order-error": []TagStr{
		TagStr{language.AmericanEnglish, "Please select some products."},
		TagStr{language.German, "Bitte wähle eines oder mehrere Produkte aus."},
	},
	"product": []TagStr{
		TagStr{language.AmericanEnglish, "Product"},
		TagStr{language.German, "Produkt"},
	},
	"amount": []TagStr{
		TagStr{language.AmericanEnglish, "Amount"},
		TagStr{language.German, "Anzahl"},
	},
	"in-stock": []TagStr{
		TagStr{language.AmericanEnglish, "In Stock"},
		TagStr{language.German, "Verfügbar"},
	},
	"item-price": []TagStr{
		TagStr{language.AmericanEnglish, "Item Price"},
		TagStr{language.German, "Einzelpreis"},
	},
	"click-more": []TagStr{
		TagStr{language.AmericanEnglish, "click here to read more"},
		TagStr{language.German, "klicke hier für mehr Infos"},
	},
	"country-error": []TagStr{
		TagStr{language.AmericanEnglish, "Please select a country from the list."},
		TagStr{language.German, "Bitte wähle ein Land aus der Liste aus."},
	},
	"captcha-label": []TagStr{
		TagStr{language.AmericanEnglish, "Please type the digits in order to solve the captcha:"},
		TagStr{language.German, "Bitte tippe die Ziffern ab, um das Captcha zu lösen:"},
	},
	"captcha-error": []TagStr{
		TagStr{language.AmericanEnglish, "Please type the digits correctly."},
		TagStr{language.German, "Bitte tippe das Captcha korrekt ab."},
	},
	"captcha-reload": []TagStr{
		TagStr{language.AmericanEnglish, "Load other image (requires JavaScript)"},
		TagStr{language.German, "Anderes Bild laden (erfordert JavaScript)"},
	},
	"submit-order": []TagStr{
		TagStr{language.AmericanEnglish, "Order"},
		TagStr{language.German, "Bestellen"},
	},
	"purchase": []TagStr{
		TagStr{language.AmericanEnglish, "Order"},
		TagStr{language.German, "Bestellung"},
	},
	"status": []TagStr{
		TagStr{language.AmericanEnglish, "Status"},
		TagStr{language.German, "Status"},
	},
	"javascript-reload": []TagStr{
		TagStr{language.AmericanEnglish, "JavaScript is disabled in your browser. In order to receive updates on your order, please reload this page from time to time."},
		TagStr{language.German, "Du hast JavaScript deaktiviert. Um über Neuigkeiten informiert zu werden, lade die Seite bitte gelegentlich neu."},
	},
	"whats-next": []TagStr{
		TagStr{language.AmericanEnglish, "What's next?"},
		TagStr{language.German, "Wie geht es weiter?"},
	},
	"next-1": []TagStr{
		TagStr{language.AmericanEnglish, "Check your order."},
		TagStr{language.German, "Prüfe deine Bestellung."},
	},
	"payment": []TagStr{
		TagStr{language.AmericanEnglish, "Payment"},
		TagStr{language.German, "Bezahlung"},
	},
	"payment-btcpay": []TagStr{
		TagStr{language.AmericanEnglish, "Monero or Bitcoin"},
		TagStr{language.German, "Monero oder Bitcoin"},
	},
	"payment-btcpay-intro": []TagStr{
		TagStr{language.AmericanEnglish, "The amount must be paid completely with a single transaction within 60 minutes. As soon as your payment is confirmed on the blockchain, your voucher codes are shown. If your payment arrives too late, we have to confirm it manually. If in doubt, please contact us."},
		TagStr{language.German, "Der Betrag muss innerhalb von 60 Minuten vollständig und als einzelne Transaktion auf der angegebenen Adresse eingehen. Sobald deine Zahlung in der Blockchain bestätigt wurde, werden dir deine Gutscheincodes angezeigt. Falls deine Zahlung verspätet eintrifft, müssen wir sie manuell bestätigen. Im Zweifel kontaktiere uns bitte."},
	},
	"payment-cash": []TagStr{
		TagStr{language.AmericanEnglish, "Cash"},
		TagStr{language.German, "Bargeld"},
	},
	"payment-cash-intro": []TagStr{
		TagStr{language.AmericanEnglish, `You can send cash in an insured letter or package to our store address in Germany. Please check the cash shipment limits of your postal company (e. g. Deutsche Post "Einschreiben Wert" up to 100 Euros within Germany, or DHL Parcel up to 500 Euros). Send it to:`},
		TagStr{language.German, `Schicke uns Bargeld in einem versichertem Brief oder Paket. Bitte beachte die Höchstgrenzen deines Postunternehmens für den Bargeldversand (z. B. Deutsche Post "Einschreiben Wert" bis 100 Euro innerhalb von Deutschland, oder DHL Paket bis 500 Euro). Sende es an:`},
	},
	"payment-cash-payid": []TagStr{
		TagStr{language.AmericanEnglish, "Please include a note with this payment code:"},
		TagStr{language.German, "Bitte lege einen Zettel mit diesem Zahlungscode bei:"},
	},
	"payment-sepa": []TagStr{
		TagStr{language.AmericanEnglish, "SEPA Bank Transfer"},
		TagStr{language.German, "SEPA-Banküberweisung"},
	},
	"payment-sepa-intro": []TagStr{
		TagStr{language.AmericanEnglish, "If you have a SEPA (Single Euro Payments Area) bank account, you can do a SEPA bank transfer to our German bank account. We will see your name and bank account number on our account statement."},
		TagStr{language.German, "Falls du ein SEPA-Bankkonto (Europäischer Zahlungsraum) hast, kannst du den Betrag per SEPA-Überweisung auf unser deutsches Bankkonto überweisen. Wir werden deinen Namen und deine IBAN auf unserem Kontoauszug sehen."},
	},
	"payment-sepa-payid": []TagStr{
		TagStr{language.AmericanEnglish, "Please enter this payment code in the payment reference field:"},
		TagStr{language.German, "Gib als Verwendungszweck bitte diesen Zahlungscode an:"},
	},
	"btcpay-link": []TagStr{
		TagStr{language.AmericanEnglish, "Pay using Monero or Bitcoin"},
		TagStr{language.German, "Zur Bezahlung mit Monero oder Bitcoin"},
	},
	"your-order": []TagStr{
		TagStr{language.AmericanEnglish, "Your Order"},
		TagStr{language.German, "Deine Bestellung"},
	},
	"sum": []TagStr{
		TagStr{language.AmericanEnglish, "Sum"},
		TagStr{language.German, "Summe"},
	},
	"overall-sum": []TagStr{
		TagStr{language.AmericanEnglish, "Overall Sum"},
		TagStr{language.German, "Gesamtsumme"},
	},
	"your-goods": []TagStr{
		TagStr{language.AmericanEnglish, "Your Goods"},
		TagStr{language.German, "Deine Ware"},
	},
	"delivery-date": []TagStr{
		TagStr{language.AmericanEnglish, "Delivery Date"},
		TagStr{language.German, "Lieferdatum"},
	},
	"id": []TagStr{
		TagStr{language.AmericanEnglish, "ID"},
		TagStr{language.German, "ID"},
	},
	"code": []TagStr{
		TagStr{language.AmericanEnglish, "Code"},
		TagStr{language.German, "Code"},
	},
	"sorry-underdelivered": []TagStr{
		TagStr{language.AmericanEnglish, "You will receive the missing codes here as soon as they are in stock again. Sorry for the inconvenience."},
		TagStr{language.German, "Die fehlenden Codes erhälst du, sobald Nachschub eintroffen ist. Wir bitten die Umstände zu entschuldigen."},
	},
	"info-unpaid": []TagStr{
		TagStr{language.AmericanEnglish, "You will receive your codes as soon as you payment has arrived."},
		TagStr{language.German, "Sobald deine Zahlung bei uns eingegangen ist, erhälst du die Codes."},
	},
	"info-delete": []TagStr{
		TagStr{language.AmericanEnglish, "Current deletion date:"},
		TagStr{language.German, "Derzeitiges Löschdatum:"},
	},
	"status-new": []TagStr{
		TagStr{language.AmericanEnglish, "New: We are waiting for your payment."},
		TagStr{language.German, "Neu: Wir warten auf den Eingang deiner Zahlung."},
	},
	"status-btcpay-created": []TagStr{
		TagStr{language.AmericanEnglish, "BTCPay invoice created"},
		TagStr{language.German, "BTCPay-Rechnung erzeugt"},
	},
	"status-btcpay-expired": []TagStr{
		TagStr{language.AmericanEnglish, "BTCPay invoice expired: The BTCPay invoice has been paid late, partly or not at all. You can still pay cash or by SEPA bank transfer."},
		TagStr{language.German, "BTCPay-Rechnung abgelaufen: Die BTCPay-Rechnung wurde zu spät, unvollständig oder gar nicht bezahlt. Du kannst immer noch bar oder per SEPA-Überweisung bezahlen."},
	},
	"status-underdelivered": []TagStr{
		TagStr{language.AmericanEnglish, "Underdelivered: We have received your payment, but have gone out of stock meanwhile. You will receive the missing codes here as soon as possible. Sorry for the inconvenience."},
		TagStr{language.German, "Untergeliefert: Wir haben deine Zahlung erhalten, aber unser Vorrat wurde zwischenzeitlich ausverkauft. Die fehlenden Codes erhälst du möglichst bald. Wir bitten die Umstände zu entschuldigen."},
	},
	"status-finalized": []TagStr{
		TagStr{language.AmericanEnglish, "Finalized: Your codes have been delivered."},
		TagStr{language.German, "Abgeschlossen: Deine Codes wurden ausgeliefert."},
	},
	"error-internal": []TagStr{
		TagStr{language.AmericanEnglish, "Internal server error: "},
		TagStr{language.German, "Interner Fehler: "},
	},
	"error-purchase-not-found": []TagStr{
		TagStr{language.AmericanEnglish, "There is no such order, or it has been deleted."},
		TagStr{language.German, "Diese Bestellung wurde nicht gefunden oder bereits gelöscht."},
	},
	"country-tax-question": []TagStr{
		TagStr{language.AmericanEnglish, "Where do you live? (We have to ask that for tax reasons. It does not affect the price or the goods.)"},
		TagStr{language.German, "In welchem Land bist du ansässig? (Das müssen wir aus steuerlichen Gründen fragen. Es hat keinen Einfluss auf den Preis oder die Leistung.)"},
	},
	"country-BE": []TagStr{
		TagStr{language.AmericanEnglish, "Belgium"},
		TagStr{language.German, "Belgien"},
	},
	"country-BG": []TagStr{
		TagStr{language.AmericanEnglish, "Bulgaria"},
		TagStr{language.German, "Bulgarien"},
	},
	"country-DK": []TagStr{
		TagStr{language.AmericanEnglish, "Denmark"},
		TagStr{language.German, "Dänemark"},
	},
	"country-DE": []TagStr{
		TagStr{language.AmericanEnglish, "Germany"},
		TagStr{language.German, "Deutschland"},
	},
	"country-EE": []TagStr{
		TagStr{language.AmericanEnglish, "Estonia"},
		TagStr{language.German, "Estland"},
	},
	"country-FI": []TagStr{
		TagStr{language.AmericanEnglish, "Finland"},
		TagStr{language.German, "Finnland"},
	},
	"country-FR": []TagStr{
		TagStr{language.AmericanEnglish, "France"},
		TagStr{language.German, "Frankreich"},
	},
	"country-EL": []TagStr{
		TagStr{language.AmericanEnglish, "Greece"},
		TagStr{language.German, "Griechenland"},
	},
	"country-IE": []TagStr{
		TagStr{language.AmericanEnglish, "Ireland"},
		TagStr{language.German, "Irland"},
	},
	"country-IT": []TagStr{
		TagStr{language.AmericanEnglish, "Italy"},
		TagStr{language.German, "Italien"},
	},
	"country-HR": []TagStr{
		TagStr{language.AmericanEnglish, "Croatia"},
		TagStr{language.German, "Kroatien"},
	},
	"country-LV": []TagStr{
		TagStr{language.AmericanEnglish, "Latvia"},
		TagStr{language.German, "Lettland"},
	},
	"country-LT": []TagStr{
		TagStr{language.AmericanEnglish, "Lithuania"},
		TagStr{language.German, "Litauen"},
	},
	"country-LU": []TagStr{
		TagStr{language.AmericanEnglish, "Luxembourg"},
		TagStr{language.German, "Luxemburg"},
	},
	"country-MT": []TagStr{
		TagStr{language.AmericanEnglish, "Malta"},
		TagStr{language.German, "Malta"},
	},
	"country-NL": []TagStr{
		TagStr{language.AmericanEnglish, "Netherlands"},
		TagStr{language.German, "Niederlande"},
	},
	"country-AT": []TagStr{
		TagStr{language.AmericanEnglish, "Austria"},
		TagStr{language.German, "Österreich"},
	},
	"country-PL": []TagStr{
		TagStr{language.AmericanEnglish, "Poland"},
		TagStr{language.German, "Polen"},
	},
	"country-PT": []TagStr{
		TagStr{language.AmericanEnglish, "Portugal"},
		TagStr{language.German, "Portugal"},
	},
	"country-RO": []TagStr{
		TagStr{language.AmericanEnglish, "Romania"},
		TagStr{language.German, "Rumänien"},
	},
	"country-SE": []TagStr{
		TagStr{language.AmericanEnglish, "Sweden"},
		TagStr{language.German, "Schweden"},
	},
	"country-SK": []TagStr{
		TagStr{language.AmericanEnglish, "Slovakia"},
		TagStr{language.German, "Slowakei"},
	},
	"country-SI": []TagStr{
		TagStr{language.AmericanEnglish, "Slovenia"},
		TagStr{language.German, "Slowenien"},
	},
	"country-ES": []TagStr{
		TagStr{language.AmericanEnglish, "Spain"},
		TagStr{language.German, "Spanien"},
	},
	"country-CZ": []TagStr{
		TagStr{language.AmericanEnglish, "Czechia"},
		TagStr{language.German, "Tschechien"},
	},
	"country-HU": []TagStr{
		TagStr{language.AmericanEnglish, "Hungary"},
		TagStr{language.German, "Ungarn"},
	},
	"country-CY": []TagStr{
		TagStr{language.AmericanEnglish, "Cyprus"},
		TagStr{language.German, "Zypern"},
	},
	"non-EU": []TagStr{
		TagStr{language.AmericanEnglish, "Not in the European Union"},
		TagStr{language.German, "Außerhalb der Europäischen Union"},
	},
	"EU": []TagStr{
		TagStr{language.AmericanEnglish, "European Union"},
		TagStr{language.German, "Europäische Union"},
	},

	// We're using this translation mechanism for technical default values too:

	"default-country": []TagStr{
		TagStr{language.AmericanEnglish, "non-EU"},
		TagStr{language.German, "DE"},
	},
	// values taken from our BTCPay instance at https://pay.example.com/misc/lang
	"btcpay-defaultlanguage": []TagStr{
		TagStr{language.AmericanEnglish, "en"},
		TagStr{language.German, "de-DE"},
	},
}

// Language is any string. It will be matched by golang.org/x/text/language.Make and golang.org/x/text/language.NewMatcher.
type Language string

// GetLanguage returns the "lang" GET parameter or, if not present, the Accept-Language header value.
// No matching is performed.
func GetLanguage(r *http.Request) Language {
	if lang := r.URL.Query().Get("lang"); lang != "" {
		if len(lang) > 35 {
			lang = lang[:35] // max length of language tag
		}
		return Language(lang)
	}
	return Language(r.Header.Get("Accept-Language"))
}

func (lang Language) Translate(key string, args ...interface{}) string {
	item, ok := translations[key]
	if !ok {
		// key not found, create language tag and print key
		return message.NewPrinter(language.Make(string(lang))).Sprintf(key, args...)
	}
	// choose language tag from list of translations
	langs := make([]language.Tag, len(item))
	for i := range item {
		langs[i] = item[i].Tag
	}
	tag, i := language.MatchStrings(language.NewMatcher(langs), string(lang))
	return message.NewPrinter(tag).Sprintf(item[i].Str, args...)
}

func (lang Language) TranslateEUCountries() []IDName {
	result := make([]IDName, len(euCountryCodes))
	for i := range euCountryCodes {
		result[i].ID = euCountryCodes[i]
		result[i].Name = lang.Translate("country-" + euCountryCodes[i])
	}
	// sort with diacritics etc. in the right order
	collator := collate.New(language.Und, collate.Loose)
	sort.Slice(result, func(i, j int) bool {
		return collator.CompareString(result[i].Name, result[j].Name) < 0
	})
	return result
}

func IsCountryCode(s string) bool {
	if s == "non-EU" {
		return true
	}
	for _, euCode := range euCountryCodes {
		if euCode == s {
			return true
		}
	}
	return false
}
