package html

import (
	"net/http"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// https://ec.europa.eu/eurostat/statistics-explained/index.php?title=Glossary:Country_codes/de
//
// TODO move to root/domain package
var EUCountryCodes = [...]string{"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI", "FR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK"}

// ISO-3166-1
//
// TODO move to root/domain package
var ISOCountryCodes = [...]string{
	"AF", "AX", "AL", "DZ", "AS", "AD", "AO", "AI", "AQ", "AG", "AR", "AM", "AW", "AU", "AT", "AZ", "BS", "BH", "BD", "BB", "BY", "BE", "BZ", "BJ", "BM", "BT", "BO", "BQ", "BA", "BW", "BV", "BR", "IO", "BN", "BG", "BF", "BI", "KH", "CM", "CA", "CV", "KY", "CF", "TD", "CL", "CN", "CX", "CC", "CO", "KM", "CG", "CD", "CK", "CR", "CI", "HR", "CU", "CW", "CY", "CZ", "DK", "DJ", "DM", "DO", "EC", "EG", "SV", "GQ", "ER", "EE", "ET", "FK", "FO", "FJ", "FI", "FR", "GF", "PF", "TF", "GA", "GM", "GE", "DE", "GH", "GI", "GR", "GL", "GD", "GP", "GU", "GT", "GG", "GN", "GW", "GY", "HT", "HM", "VA", "HN", "HK", "HU", "IS", "IN", "ID", "IR", "IQ", "IE", "IM", "IL", "IT", "JM", "JP", "JE", "JO", "KZ", "KE", "KI", "KP", "KR", "KW", "KG", "LA", "LV", "LB", "LS", "LR", "LY", "LI", "LT", "LU", "MO", "MK", "MG", "MW", "MY", "MV", "ML", "MT", "MH", "MQ", "MR", "MU", "YT", "MX", "FM", "MD", "MC", "MN", "ME", "MS", "MA", "MZ", "MM", "NA", "NR", "NP", "NL", "NC", "NZ", "NI", "NE", "NG", "NU", "NF", "MP", "NO", "OM", "PK", "PW", "PS", "PA", "PG", "PY", "PE", "PH", "PN", "PL", "PT", "PR", "QA", "RE", "RO", "RU", "RW", "BL", "SH", "KN", "LC", "MF", "PM", "VC", "WS", "SM", "ST", "SA", "SN", "RS", "SC", "SL", "SG", "SX", "SK", "SI", "SB", "SO", "ZA", "GS", "SS", "ES", "LK", "SD", "SR", "SJ", "SZ", "SE", "CH", "SY", "TW", "TJ", "TZ", "TH", "TL", "TG", "TK", "TO", "TT", "TN", "TR", "TM", "TC", "TV", "UG", "UA", "AE", "GB", "US", "UM", "UY", "UZ", "VU", "VE", "VN", "VG", "VI", "WF", "EH", "YE", "ZM", "ZW"}

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
		TagStr{language.German, "Vorrätig"},
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

	"default-eu-country": []TagStr{
		TagStr{language.AmericanEnglish, "non-EU"},
		TagStr{language.German, "DE"},
	},
	"default-iso-country": []TagStr{
		TagStr{language.AmericanEnglish, "US"},
		TagStr{language.German, "DE"},
	},
	// values taken from our BTCPay instance at https://pay.example.com/misc/lang
	"btcpay-defaultlanguage": []TagStr{
		TagStr{language.AmericanEnglish, "en"},
		TagStr{language.German, "de-DE"},
	},

	// ISO-3166-1 country list
	"AD": []TagStr{
		TagStr{language.AmericanEnglish, "Andorra"},
		TagStr{language.German, "Andorra"},
	},
	"AE": []TagStr{
		TagStr{language.AmericanEnglish, "United Arab Emirates"},
		TagStr{language.German, "Vereinigte Arabische Emirate"},
	},
	"AF": []TagStr{
		TagStr{language.AmericanEnglish, "Afghanistan"},
		TagStr{language.German, "Afghanistan"},
	},
	"AG": []TagStr{
		TagStr{language.AmericanEnglish, "Antigua and Barbuda"},
		TagStr{language.German, "Antigua und Barbuda"},
	},
	"AI": []TagStr{
		TagStr{language.AmericanEnglish, "Anguilla"},
		TagStr{language.German, "Anguilla"},
	},
	"AL": []TagStr{
		TagStr{language.AmericanEnglish, "Albania"},
		TagStr{language.German, "Albanien"},
	},
	"AM": []TagStr{
		TagStr{language.AmericanEnglish, "Armenia"},
		TagStr{language.German, "Armenien"},
	},
	"AO": []TagStr{
		TagStr{language.AmericanEnglish, "Angola"},
		TagStr{language.German, "Angola"},
	},
	"AQ": []TagStr{
		TagStr{language.AmericanEnglish, "Antarctica"},
		TagStr{language.German, "Antarktis"},
	},
	"AR": []TagStr{
		TagStr{language.AmericanEnglish, "Argentina"},
		TagStr{language.German, "Argentinien"},
	},
	"AS": []TagStr{
		TagStr{language.AmericanEnglish, "American Samoa"},
		TagStr{language.German, "Samoa"},
	},
	"AT": []TagStr{
		TagStr{language.AmericanEnglish, "Austria"},
		TagStr{language.German, "Österreich"},
	},
	"AU": []TagStr{
		TagStr{language.AmericanEnglish, "Australia"},
		TagStr{language.German, "Australien"},
	},
	"AW": []TagStr{
		TagStr{language.AmericanEnglish, "Aruba"},
		TagStr{language.German, "Aruba"},
	},
	"AX": []TagStr{
		TagStr{language.AmericanEnglish, "Åland Islands"},
		TagStr{language.German, "Åland"},
	},
	"AZ": []TagStr{
		TagStr{language.AmericanEnglish, "Azerbaijan"},
		TagStr{language.German, "Aserbaidschan"},
	},
	"BA": []TagStr{
		TagStr{language.AmericanEnglish, "Bosnia and Herzegovina"},
		TagStr{language.German, "Bosnien-Herzegowina"},
	},
	"BB": []TagStr{
		TagStr{language.AmericanEnglish, "Barbados"},
		TagStr{language.German, "Barbados"},
	},
	"BD": []TagStr{
		TagStr{language.AmericanEnglish, "Bangladesh"},
		TagStr{language.German, "Bangladesh"},
	},
	"BE": []TagStr{
		TagStr{language.AmericanEnglish, "Belgium"},
		TagStr{language.German, "Belgien"},
	},
	"BF": []TagStr{
		TagStr{language.AmericanEnglish, "Burkina Faso"},
		TagStr{language.German, "Burkina Faso"},
	},
	"BG": []TagStr{
		TagStr{language.AmericanEnglish, "Bulgaria"},
		TagStr{language.German, "Bulgarien"},
	},
	"BH": []TagStr{
		TagStr{language.AmericanEnglish, "Bahrain"},
		TagStr{language.German, "Bahrain"},
	},
	"BI": []TagStr{
		TagStr{language.AmericanEnglish, "Burundi"},
		TagStr{language.German, "Burundi"},
	},
	"BJ": []TagStr{
		TagStr{language.AmericanEnglish, "Benin"},
		TagStr{language.German, "Benin"},
	},
	"BL": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Barthélemy"},
		TagStr{language.German, "Saint-Barthélemy"},
	},
	"BM": []TagStr{
		TagStr{language.AmericanEnglish, "Bermuda"},
		TagStr{language.German, "Bermudas"},
	},
	"BN": []TagStr{
		TagStr{language.AmericanEnglish, "Brunei Darussalam"},
		TagStr{language.German, "Brunei"},
	},
	"BO": []TagStr{
		TagStr{language.AmericanEnglish, "Bolivia, Plurinational State of"},
		TagStr{language.German, "Bolivien"},
	},
	"BQ": []TagStr{
		TagStr{language.AmericanEnglish, "Bonaire, Sint Eustatius and Saba"},
		TagStr{language.German, "Karibische Niederlande"},
	},
	"BR": []TagStr{
		TagStr{language.AmericanEnglish, "Brazil"},
		TagStr{language.German, "Brasilien"},
	},
	"BS": []TagStr{
		TagStr{language.AmericanEnglish, "Bahamas"},
		TagStr{language.German, "Bahamas"},
	},
	"BT": []TagStr{
		TagStr{language.AmericanEnglish, "Bhutan"},
		TagStr{language.German, "Bhutan"},
	},
	"BW": []TagStr{
		TagStr{language.AmericanEnglish, "Botswana"},
		TagStr{language.German, "Botswana"},
	},
	"BY": []TagStr{
		TagStr{language.AmericanEnglish, "Belarus"},
		TagStr{language.German, "Weissrussland"},
	},
	"BZ": []TagStr{
		TagStr{language.AmericanEnglish, "Belize"},
		TagStr{language.German, "Belize"},
	},
	"CA": []TagStr{
		TagStr{language.AmericanEnglish, "Canada"},
		TagStr{language.German, "Kanada"},
	},
	"CC": []TagStr{
		TagStr{language.AmericanEnglish, "Cocos (Keeling) Islands"},
		TagStr{language.German, "Kokosinseln"},
	},
	"CD": []TagStr{
		TagStr{language.AmericanEnglish, "Congo, the Democratic Republic of the"},
		TagStr{language.German, "Kongo, Demokratische Republik"},
	},
	"CF": []TagStr{
		TagStr{language.AmericanEnglish, "Central African Republic"},
		TagStr{language.German, "Zentralafrikanische Republik"},
	},
	"CG": []TagStr{
		TagStr{language.AmericanEnglish, "Congo"},
		TagStr{language.German, "Kongo"},
	},
	"CH": []TagStr{
		TagStr{language.AmericanEnglish, "Switzerland"},
		TagStr{language.German, "Schweiz"},
	},
	"CI": []TagStr{
		TagStr{language.AmericanEnglish, "Côte d'Ivoire"},
		TagStr{language.German, "Elfenbeinküste"},
	},
	"CK": []TagStr{
		TagStr{language.AmericanEnglish, "Cook Islands"},
		TagStr{language.German, "Cookinseln"},
	},
	"CL": []TagStr{
		TagStr{language.AmericanEnglish, "Chile"},
		TagStr{language.German, "Chile"},
	},
	"CM": []TagStr{
		TagStr{language.AmericanEnglish, "Cameroon"},
		TagStr{language.German, "Kamerun"},
	},
	"CN": []TagStr{
		TagStr{language.AmericanEnglish, "China"},
		TagStr{language.German, "China"},
	},
	"CO": []TagStr{
		TagStr{language.AmericanEnglish, "Colombia"},
		TagStr{language.German, "Kolumbien"},
	},
	"CR": []TagStr{
		TagStr{language.AmericanEnglish, "Costa Rica"},
		TagStr{language.German, "Costa Rica"},
	},
	"CU": []TagStr{
		TagStr{language.AmericanEnglish, "Cuba"},
		TagStr{language.German, "Kuba"},
	},
	"CV": []TagStr{
		TagStr{language.AmericanEnglish, "Cape Verde"},
		TagStr{language.German, "Kap Verde"},
	},
	"CW": []TagStr{
		TagStr{language.AmericanEnglish, "Curaçao"},
		TagStr{language.German, "Curaçao"},
	},
	"CX": []TagStr{
		TagStr{language.AmericanEnglish, "Christmas Island"},
		TagStr{language.German, "Christmas Island"},
	},
	"CY": []TagStr{
		TagStr{language.AmericanEnglish, "Cyprus"},
		TagStr{language.German, "Zypern"},
	},
	"CZ": []TagStr{
		TagStr{language.AmericanEnglish, "Czech Republic"},
		TagStr{language.German, "Tschechische Republik"},
	},
	"DE": []TagStr{
		TagStr{language.AmericanEnglish, "Germany"},
		TagStr{language.German, "Deutschland"},
	},
	"DJ": []TagStr{
		TagStr{language.AmericanEnglish, "Djibouti"},
		TagStr{language.German, "Djibuti"},
	},
	"DK": []TagStr{
		TagStr{language.AmericanEnglish, "Denmark"},
		TagStr{language.German, "Dänemark"},
	},
	"DM": []TagStr{
		TagStr{language.AmericanEnglish, "Dominica"},
		TagStr{language.German, "Dominika"},
	},
	"DO": []TagStr{
		TagStr{language.AmericanEnglish, "Dominican Republic"},
		TagStr{language.German, "Dominikanische Republik"},
	},
	"DZ": []TagStr{
		TagStr{language.AmericanEnglish, "Algeria"},
		TagStr{language.German, "Algerien"},
	},
	"EC": []TagStr{
		TagStr{language.AmericanEnglish, "Ecuador"},
		TagStr{language.German, "Ecuador"},
	},
	"EE": []TagStr{
		TagStr{language.AmericanEnglish, "Estonia"},
		TagStr{language.German, "Estland"},
	},
	"EG": []TagStr{
		TagStr{language.AmericanEnglish, "Egypt"},
		TagStr{language.German, "Ägypten"},
	},
	"EH": []TagStr{
		TagStr{language.AmericanEnglish, "Western Sahara"},
		TagStr{language.German, "Westsahara"},
	},
	"ER": []TagStr{
		TagStr{language.AmericanEnglish, "Eritrea"},
		TagStr{language.German, "Eritrea"},
	},
	"ES": []TagStr{
		TagStr{language.AmericanEnglish, "Spain"},
		TagStr{language.German, "Spanien"},
	},
	"ET": []TagStr{
		TagStr{language.AmericanEnglish, "Ethiopia"},
		TagStr{language.German, "Äthiopien"},
	},
	"FI": []TagStr{
		TagStr{language.AmericanEnglish, "Finland"},
		TagStr{language.German, "Finnland"},
	},
	"FJ": []TagStr{
		TagStr{language.AmericanEnglish, "Fiji"},
		TagStr{language.German, "Fidschi"},
	},
	"FK": []TagStr{
		TagStr{language.AmericanEnglish, "Falkland Islands (Malvinas)"},
		TagStr{language.German, "Falklandinseln"},
	},
	"FM": []TagStr{
		TagStr{language.AmericanEnglish, "Micronesia, Federated States of"},
		TagStr{language.German, "Mikronesien"},
	},
	"FO": []TagStr{
		TagStr{language.AmericanEnglish, "Faroe Islands"},
		TagStr{language.German, "Färöer Inseln"},
	},
	"FR": []TagStr{
		TagStr{language.AmericanEnglish, "France"},
		TagStr{language.German, "Frankreich"},
	},
	"GA": []TagStr{
		TagStr{language.AmericanEnglish, "Gabon"},
		TagStr{language.German, "Gabun"},
	},
	"GB": []TagStr{
		TagStr{language.AmericanEnglish, "United Kingdom"},
		TagStr{language.German, "Großbritannien (UK)"},
	},
	"GD": []TagStr{
		TagStr{language.AmericanEnglish, "Grenada"},
		TagStr{language.German, "Grenada"},
	},
	"GE": []TagStr{
		TagStr{language.AmericanEnglish, "Georgia"},
		TagStr{language.German, "Georgien"},
	},
	"GF": []TagStr{
		TagStr{language.AmericanEnglish, "French Guiana"},
		TagStr{language.German, "Französisch-Guayana"},
	},
	"GG": []TagStr{
		TagStr{language.AmericanEnglish, "Guernsey"},
		TagStr{language.German, "Guernsey"},
	},
	"GH": []TagStr{
		TagStr{language.AmericanEnglish, "Ghana"},
		TagStr{language.German, "Ghana"},
	},
	"GI": []TagStr{
		TagStr{language.AmericanEnglish, "Gibraltar"},
		TagStr{language.German, "Gibraltar"},
	},
	"GL": []TagStr{
		TagStr{language.AmericanEnglish, "Greenland"},
		TagStr{language.German, "Grönland"},
	},
	"GM": []TagStr{
		TagStr{language.AmericanEnglish, "Gambia"},
		TagStr{language.German, "Gambia"},
	},
	"GN": []TagStr{
		TagStr{language.AmericanEnglish, "Guinea"},
		TagStr{language.German, "Guinea"},
	},
	"GP": []TagStr{
		TagStr{language.AmericanEnglish, "Guadeloupe"},
		TagStr{language.German, "Guadeloupe"},
	},
	"GQ": []TagStr{
		TagStr{language.AmericanEnglish, "Equatorial Guinea"},
		TagStr{language.German, "Äquatorialguinea"},
	},
	"GR": []TagStr{
		TagStr{language.AmericanEnglish, "Greece"},
		TagStr{language.German, "Griechenland"},
	},
	"GS": []TagStr{
		TagStr{language.AmericanEnglish, "South Georgia and the South Sandwich Islands"},
		TagStr{language.German, "Südgeorgien und die Südlichen Sandwichinseln"},
	},
	"GT": []TagStr{
		TagStr{language.AmericanEnglish, "Guatemala"},
		TagStr{language.German, "Guatemala"},
	},
	"GU": []TagStr{
		TagStr{language.AmericanEnglish, "Guam"},
		TagStr{language.German, "Guam"},
	},
	"GW": []TagStr{
		TagStr{language.AmericanEnglish, "Guinea-Bissau"},
		TagStr{language.German, "Guinea-Bissau"},
	},
	"GY": []TagStr{
		TagStr{language.AmericanEnglish, "Guyana"},
		TagStr{language.German, "Guyana"},
	},
	"HK": []TagStr{
		TagStr{language.AmericanEnglish, "Hong Kong"},
		TagStr{language.German, "Hongkong"},
	},
	"HM": []TagStr{
		TagStr{language.AmericanEnglish, "Heard Island and McDonald Islands"},
		TagStr{language.German, "Heard und McDonaldinseln"},
	},
	"HN": []TagStr{
		TagStr{language.AmericanEnglish, "Honduras"},
		TagStr{language.German, "Honduras"},
	},
	"HR": []TagStr{
		TagStr{language.AmericanEnglish, "Croatia"},
		TagStr{language.German, "Kroatien"},
	},
	"HT": []TagStr{
		TagStr{language.AmericanEnglish, "Haiti"},
		TagStr{language.German, "Haiti"},
	},
	"HU": []TagStr{
		TagStr{language.AmericanEnglish, "Hungary"},
		TagStr{language.German, "Ungarn"},
	},
	"ID": []TagStr{
		TagStr{language.AmericanEnglish, "Indonesia"},
		TagStr{language.German, "Indonesien"},
	},
	"IE": []TagStr{
		TagStr{language.AmericanEnglish, "Ireland"},
		TagStr{language.German, "Irland"},
	},
	"IL": []TagStr{
		TagStr{language.AmericanEnglish, "Israel"},
		TagStr{language.German, "Israel"},
	},
	"IM": []TagStr{
		TagStr{language.AmericanEnglish, "Isle of Man"},
		TagStr{language.German, "Isle of Man"},
	},
	"IN": []TagStr{
		TagStr{language.AmericanEnglish, "India"},
		TagStr{language.German, "Indien"},
	},
	"IO": []TagStr{
		TagStr{language.AmericanEnglish, "British Indian Ocean Territory"},
		TagStr{language.German, "Britisch-Indischer Ozean"},
	},
	"IQ": []TagStr{
		TagStr{language.AmericanEnglish, "Iraq"},
		TagStr{language.German, "Irak"},
	},
	"IR": []TagStr{
		TagStr{language.AmericanEnglish, "Iran, Islamic Republic of"},
		TagStr{language.German, "Iran"},
	},
	"IS": []TagStr{
		TagStr{language.AmericanEnglish, "Iceland"},
		TagStr{language.German, "Island"},
	},
	"IT": []TagStr{
		TagStr{language.AmericanEnglish, "Italy"},
		TagStr{language.German, "Italien"},
	},
	"JE": []TagStr{
		TagStr{language.AmericanEnglish, "Jersey"},
		TagStr{language.German, "Jersey"},
	},
	"JM": []TagStr{
		TagStr{language.AmericanEnglish, "Jamaica"},
		TagStr{language.German, "Jamaika"},
	},
	"JO": []TagStr{
		TagStr{language.AmericanEnglish, "Jordan"},
		TagStr{language.German, "Jordanien"},
	},
	"JP": []TagStr{
		TagStr{language.AmericanEnglish, "Japan"},
		TagStr{language.German, "Japan"},
	},
	"KE": []TagStr{
		TagStr{language.AmericanEnglish, "Kenya"},
		TagStr{language.German, "Kenia"},
	},
	"KG": []TagStr{
		TagStr{language.AmericanEnglish, "Kyrgyzstan"},
		TagStr{language.German, "Kirgisistan"},
	},
	"KH": []TagStr{
		TagStr{language.AmericanEnglish, "Cambodia"},
		TagStr{language.German, "Kambodscha"},
	},
	"KI": []TagStr{
		TagStr{language.AmericanEnglish, "Kiribati"},
		TagStr{language.German, "Kiribati"},
	},
	"KM": []TagStr{
		TagStr{language.AmericanEnglish, "Comoros"},
		TagStr{language.German, "Komoren"},
	},
	"KN": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Kitts and Nevis"},
		TagStr{language.German, "St. Kitts und Nevis"},
	},
	"KP": []TagStr{
		TagStr{language.AmericanEnglish, "Korea, Democratic People's Republic of"},
		TagStr{language.German, "Nordkorea"},
	},
	"KR": []TagStr{
		TagStr{language.AmericanEnglish, "Korea, Republic of"},
		TagStr{language.German, "Südkorea"},
	},
	"KW": []TagStr{
		TagStr{language.AmericanEnglish, "Kuwait"},
		TagStr{language.German, "Kuwait"},
	},
	"KY": []TagStr{
		TagStr{language.AmericanEnglish, "Cayman Islands"},
		TagStr{language.German, "Kaimaninseln"},
	},
	"KZ": []TagStr{
		TagStr{language.AmericanEnglish, "Kazakhstan"},
		TagStr{language.German, "Kasachstan"},
	},
	"LA": []TagStr{
		TagStr{language.AmericanEnglish, "Lao People's Democratic Republic"},
		TagStr{language.German, "Laos"},
	},
	"LB": []TagStr{
		TagStr{language.AmericanEnglish, "Lebanon"},
		TagStr{language.German, "Libanon"},
	},
	"LC": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Lucia"},
		TagStr{language.German, "Saint Lucia"},
	},
	"LI": []TagStr{
		TagStr{language.AmericanEnglish, "Liechtenstein"},
		TagStr{language.German, "Liechtenstein"},
	},
	"LK": []TagStr{
		TagStr{language.AmericanEnglish, "Sri Lanka"},
		TagStr{language.German, "Sri Lanka"},
	},
	"LR": []TagStr{
		TagStr{language.AmericanEnglish, "Liberia"},
		TagStr{language.German, "Liberia"},
	},
	"LS": []TagStr{
		TagStr{language.AmericanEnglish, "Lesotho"},
		TagStr{language.German, "Lesotho"},
	},
	"LT": []TagStr{
		TagStr{language.AmericanEnglish, "Lithuania"},
		TagStr{language.German, "Litauen"},
	},
	"LU": []TagStr{
		TagStr{language.AmericanEnglish, "Luxembourg"},
		TagStr{language.German, "Luxemburg"},
	},
	"LV": []TagStr{
		TagStr{language.AmericanEnglish, "Latvia"},
		TagStr{language.German, "Lettland"},
	},
	"LY": []TagStr{
		TagStr{language.AmericanEnglish, "Libya"},
		TagStr{language.German, "Libyen"},
	},
	"MA": []TagStr{
		TagStr{language.AmericanEnglish, "Morocco"},
		TagStr{language.German, "Marokko"},
	},
	"MC": []TagStr{
		TagStr{language.AmericanEnglish, "Monaco"},
		TagStr{language.German, "Monaco"},
	},
	"MD": []TagStr{
		TagStr{language.AmericanEnglish, "Moldova, Republic of"},
		TagStr{language.German, "Moldavien"},
	},
	"ME": []TagStr{
		TagStr{language.AmericanEnglish, "Montenegro"},
		TagStr{language.German, "Montenegro "},
	},
	"MF": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Martin (French part)"},
		TagStr{language.German, "Saint-Martin"},
	},
	"MG": []TagStr{
		TagStr{language.AmericanEnglish, "Madagascar"},
		TagStr{language.German, "Madagaskar"},
	},
	"MH": []TagStr{
		TagStr{language.AmericanEnglish, "Marshall Islands"},
		TagStr{language.German, "Marshallinseln"},
	},
	"MK": []TagStr{
		TagStr{language.AmericanEnglish, "Macedonia, the Former Yugoslav Republic of"},
		TagStr{language.German, "Mazedonien"},
	},
	"ML": []TagStr{
		TagStr{language.AmericanEnglish, "Mali"},
		TagStr{language.German, "Mali"},
	},
	"MM": []TagStr{
		TagStr{language.AmericanEnglish, "Myanmar"},
		TagStr{language.German, "Birma"},
	},
	"MN": []TagStr{
		TagStr{language.AmericanEnglish, "Mongolia"},
		TagStr{language.German, "Mongolei"},
	},
	"MO": []TagStr{
		TagStr{language.AmericanEnglish, "Macao"},
		TagStr{language.German, "Macao"},
	},
	"MP": []TagStr{
		TagStr{language.AmericanEnglish, "Northern Mariana Islands"},
		TagStr{language.German, "Marianen"},
	},
	"MQ": []TagStr{
		TagStr{language.AmericanEnglish, "Martinique"},
		TagStr{language.German, "Martinique"},
	},
	"MR": []TagStr{
		TagStr{language.AmericanEnglish, "Mauritania"},
		TagStr{language.German, "Mauretanien"},
	},
	"MS": []TagStr{
		TagStr{language.AmericanEnglish, "Montserrat"},
		TagStr{language.German, "Montserrat"},
	},
	"MT": []TagStr{
		TagStr{language.AmericanEnglish, "Malta"},
		TagStr{language.German, "Malta"},
	},
	"MU": []TagStr{
		TagStr{language.AmericanEnglish, "Mauritius"},
		TagStr{language.German, "Mauritius"},
	},
	"MV": []TagStr{
		TagStr{language.AmericanEnglish, "Maldives"},
		TagStr{language.German, "Malediven"},
	},
	"MW": []TagStr{
		TagStr{language.AmericanEnglish, "Malawi"},
		TagStr{language.German, "Malawi"},
	},
	"MX": []TagStr{
		TagStr{language.AmericanEnglish, "Mexico"},
		TagStr{language.German, "Mexiko"},
	},
	"MY": []TagStr{
		TagStr{language.AmericanEnglish, "Malaysia"},
		TagStr{language.German, "Malaysia"},
	},
	"MZ": []TagStr{
		TagStr{language.AmericanEnglish, "Mozambique"},
		TagStr{language.German, "Mocambique"},
	},
	"NA": []TagStr{
		TagStr{language.AmericanEnglish, "Namibia"},
		TagStr{language.German, "Namibia"},
	},
	"NC": []TagStr{
		TagStr{language.AmericanEnglish, "New Caledonia"},
		TagStr{language.German, "Neukaledonien"},
	},
	"NE": []TagStr{
		TagStr{language.AmericanEnglish, "Niger"},
		TagStr{language.German, "Niger"},
	},
	"NF": []TagStr{
		TagStr{language.AmericanEnglish, "Norfolk Island"},
		TagStr{language.German, "Norfolkinsel"},
	},
	"NG": []TagStr{
		TagStr{language.AmericanEnglish, "Nigeria"},
		TagStr{language.German, "Nigeria"},
	},
	"NI": []TagStr{
		TagStr{language.AmericanEnglish, "Nicaragua"},
		TagStr{language.German, "Nicaragua"},
	},
	"NL": []TagStr{
		TagStr{language.AmericanEnglish, "Netherlands"},
		TagStr{language.German, "Niederlande"},
	},
	"NO": []TagStr{
		TagStr{language.AmericanEnglish, "Norway"},
		TagStr{language.German, "Norwegen"},
	},
	"NP": []TagStr{
		TagStr{language.AmericanEnglish, "Nepal"},
		TagStr{language.German, "Nepal"},
	},
	"NR": []TagStr{
		TagStr{language.AmericanEnglish, "Nauru"},
		TagStr{language.German, "Nauru"},
	},
	"NU": []TagStr{
		TagStr{language.AmericanEnglish, "Niue"},
		TagStr{language.German, "Niue"},
	},
	"NZ": []TagStr{
		TagStr{language.AmericanEnglish, "New Zealand"},
		TagStr{language.German, "Neuseeland"},
	},
	"OM": []TagStr{
		TagStr{language.AmericanEnglish, "Oman"},
		TagStr{language.German, "Oman"},
	},
	"PA": []TagStr{
		TagStr{language.AmericanEnglish, "Panama"},
		TagStr{language.German, "Panama"},
	},
	"PE": []TagStr{
		TagStr{language.AmericanEnglish, "Peru"},
		TagStr{language.German, "Peru"},
	},
	"PF": []TagStr{
		TagStr{language.AmericanEnglish, "French Polynesia"},
		TagStr{language.German, "Französisch-Polynesien"},
	},
	"PG": []TagStr{
		TagStr{language.AmericanEnglish, "Papua New Guinea"},
		TagStr{language.German, "Papua-Neuguinea"},
	},
	"PH": []TagStr{
		TagStr{language.AmericanEnglish, "Philippines"},
		TagStr{language.German, "Philippinen"},
	},
	"PK": []TagStr{
		TagStr{language.AmericanEnglish, "Pakistan"},
		TagStr{language.German, "Pakistan"},
	},
	"PL": []TagStr{
		TagStr{language.AmericanEnglish, "Poland"},
		TagStr{language.German, "Polen"},
	},
	"PM": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Pierre and Miquelon"},
		TagStr{language.German, "Saint-Pierre und Miquelon"},
	},
	"PN": []TagStr{
		TagStr{language.AmericanEnglish, "Pitcairn"},
		TagStr{language.German, "Pitcairn"},
	},
	"PR": []TagStr{
		TagStr{language.AmericanEnglish, "Puerto Rico"},
		TagStr{language.German, "Puerto Rico"},
	},
	"PS": []TagStr{
		TagStr{language.AmericanEnglish, "Palestine, State of"},
		TagStr{language.German, "Palästina"},
	},
	"PT": []TagStr{
		TagStr{language.AmericanEnglish, "Portugal"},
		TagStr{language.German, "Portugal"},
	},
	"PW": []TagStr{
		TagStr{language.AmericanEnglish, "Palau"},
		TagStr{language.German, "Palau"},
	},
	"PY": []TagStr{
		TagStr{language.AmericanEnglish, "Paraguay"},
		TagStr{language.German, "Paraguay"},
	},
	"QA": []TagStr{
		TagStr{language.AmericanEnglish, "Qatar"},
		TagStr{language.German, "Qatar"},
	},
	"RE": []TagStr{
		TagStr{language.AmericanEnglish, "Réunion"},
		TagStr{language.German, "Réunion"},
	},
	"RO": []TagStr{
		TagStr{language.AmericanEnglish, "Romania"},
		TagStr{language.German, "Rumänien"},
	},
	"RS": []TagStr{
		TagStr{language.AmericanEnglish, "Serbia"},
		TagStr{language.German, "Serbien"},
	},
	"RU": []TagStr{
		TagStr{language.AmericanEnglish, "Russian Federation"},
		TagStr{language.German, "Russland"},
	},
	"RW": []TagStr{
		TagStr{language.AmericanEnglish, "Rwanda"},
		TagStr{language.German, "Ruanda"},
	},
	"SA": []TagStr{
		TagStr{language.AmericanEnglish, "Saudi Arabia"},
		TagStr{language.German, "Saudi-Arabien"},
	},
	"SB": []TagStr{
		TagStr{language.AmericanEnglish, "Solomon Islands"},
		TagStr{language.German, "Salomon-Inseln"},
	},
	"SC": []TagStr{
		TagStr{language.AmericanEnglish, "Seychelles"},
		TagStr{language.German, "Seychellen"},
	},
	"SD": []TagStr{
		TagStr{language.AmericanEnglish, "Sudan"},
		TagStr{language.German, "Sudan"},
	},
	"SE": []TagStr{
		TagStr{language.AmericanEnglish, "Sweden"},
		TagStr{language.German, "Schweden"},
	},
	"SG": []TagStr{
		TagStr{language.AmericanEnglish, "Singapore"},
		TagStr{language.German, "Singapur"},
	},
	"SH": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Helena, Ascension and Tristan da Cunha"},
		TagStr{language.German, "St. Helena"},
	},
	"SI": []TagStr{
		TagStr{language.AmericanEnglish, "Slovenia"},
		TagStr{language.German, "Slowenien"},
	},
	"SJ": []TagStr{
		TagStr{language.AmericanEnglish, "Svalbard and Jan Mayen"},
		TagStr{language.German, "Svalbard und Jan Mayen Islands"},
	},
	"SK": []TagStr{
		TagStr{language.AmericanEnglish, "Slovakia"},
		TagStr{language.German, "Slowakei"},
	},
	"SL": []TagStr{
		TagStr{language.AmericanEnglish, "Sierra Leone"},
		TagStr{language.German, "Sierra Leone"},
	},
	"SM": []TagStr{
		TagStr{language.AmericanEnglish, "San Marino"},
		TagStr{language.German, "San Marino"},
	},
	"SN": []TagStr{
		TagStr{language.AmericanEnglish, "Senegal"},
		TagStr{language.German, "Senegal"},
	},
	"SO": []TagStr{
		TagStr{language.AmericanEnglish, "Somalia"},
		TagStr{language.German, "Somalia"},
	},
	"SR": []TagStr{
		TagStr{language.AmericanEnglish, "Suriname"},
		TagStr{language.German, "Surinam"},
	},
	"SS": []TagStr{
		TagStr{language.AmericanEnglish, "South Sudan"},
		TagStr{language.German, "Südsudan"},
	},
	"ST": []TagStr{
		TagStr{language.AmericanEnglish, "Sao Tome and Principe"},
		TagStr{language.German, "São Tomé und Príncipe"},
	},
	"SV": []TagStr{
		TagStr{language.AmericanEnglish, "El Salvador"},
		TagStr{language.German, "El Salvador"},
	},
	"SX": []TagStr{
		TagStr{language.AmericanEnglish, "Sint Maarten (Dutch part)"},
		TagStr{language.German, "Sint Maarten"},
	},
	"SY": []TagStr{
		TagStr{language.AmericanEnglish, "Syrian Arab Republic"},
		TagStr{language.German, "Syrien"},
	},
	"SZ": []TagStr{
		TagStr{language.AmericanEnglish, "Swaziland"},
		TagStr{language.German, "Swasiland"},
	},
	"TC": []TagStr{
		TagStr{language.AmericanEnglish, "Turks and Caicos Islands"},
		TagStr{language.German, "Turks und Kaikos Inseln"},
	},
	"TD": []TagStr{
		TagStr{language.AmericanEnglish, "Chad"},
		TagStr{language.German, "Tschad"},
	},
	"TF": []TagStr{
		TagStr{language.AmericanEnglish, "French Southern Territories"},
		TagStr{language.German, "Französisches Süd-Territorium"},
	},
	"TG": []TagStr{
		TagStr{language.AmericanEnglish, "Togo"},
		TagStr{language.German, "Togo"},
	},
	"TH": []TagStr{
		TagStr{language.AmericanEnglish, "Thailand"},
		TagStr{language.German, "Thailand"},
	},
	"TJ": []TagStr{
		TagStr{language.AmericanEnglish, "Tajikistan"},
		TagStr{language.German, "Tadschikistan"},
	},
	"TK": []TagStr{
		TagStr{language.AmericanEnglish, "Tokelau"},
		TagStr{language.German, "Tokelau"},
	},
	"TL": []TagStr{
		TagStr{language.AmericanEnglish, "Timor-Leste"},
		TagStr{language.German, "Osttimor"},
	},
	"TM": []TagStr{
		TagStr{language.AmericanEnglish, "Turkmenistan"},
		TagStr{language.German, "Turkmenistan"},
	},
	"TN": []TagStr{
		TagStr{language.AmericanEnglish, "Tunisia"},
		TagStr{language.German, "Tunesien"},
	},
	"TO": []TagStr{
		TagStr{language.AmericanEnglish, "Tonga"},
		TagStr{language.German, "Tonga"},
	},
	"TR": []TagStr{
		TagStr{language.AmericanEnglish, "Turkey"},
		TagStr{language.German, "Türkei"},
	},
	"TT": []TagStr{
		TagStr{language.AmericanEnglish, "Trinidad and Tobago"},
		TagStr{language.German, "Trinidad und Tobago"},
	},
	"TV": []TagStr{
		TagStr{language.AmericanEnglish, "Tuvalu"},
		TagStr{language.German, "Tuvalu"},
	},
	"TW": []TagStr{
		TagStr{language.AmericanEnglish, "Taiwan, Province of China"},
		TagStr{language.German, "Taiwan"},
	},
	"TZ": []TagStr{
		TagStr{language.AmericanEnglish, "Tanzania, United Republic of"},
		TagStr{language.German, "Tansania"},
	},
	"UA": []TagStr{
		TagStr{language.AmericanEnglish, "Ukraine"},
		TagStr{language.German, "Ukraine"},
	},
	"UG": []TagStr{
		TagStr{language.AmericanEnglish, "Uganda"},
		TagStr{language.German, "Uganda"},
	},
	"UM": []TagStr{
		TagStr{language.AmericanEnglish, "United States Minor Outlying Islands"},
		TagStr{language.German, "United States Minor Outlying Islands"},
	},
	"US": []TagStr{
		TagStr{language.AmericanEnglish, "United States"},
		TagStr{language.German, "Vereinigte Staaten von Amerika"},
	},
	"UY": []TagStr{
		TagStr{language.AmericanEnglish, "Uruguay"},
		TagStr{language.German, "Uruguay"},
	},
	"UZ": []TagStr{
		TagStr{language.AmericanEnglish, "Uzbekistan"},
		TagStr{language.German, "Usbekistan"},
	},
	"VA": []TagStr{
		TagStr{language.AmericanEnglish, "Holy See (Vatican City State)"},
		TagStr{language.German, "Vatikan"},
	},
	"VC": []TagStr{
		TagStr{language.AmericanEnglish, "Saint Vincent and the Grenadines"},
		TagStr{language.German, "St. Vincent"},
	},
	"VE": []TagStr{
		TagStr{language.AmericanEnglish, "Venezuela, Bolivarian Republic of"},
		TagStr{language.German, "Venezuela"},
	},
	"VG": []TagStr{
		TagStr{language.AmericanEnglish, "Virgin Islands, British"},
		TagStr{language.German, "Virgin Island (Britisch)"},
	},
	"VI": []TagStr{
		TagStr{language.AmericanEnglish, "Virgin Islands, U.S."},
		TagStr{language.German, "Virgin Island (USA)"},
	},
	"VN": []TagStr{
		TagStr{language.AmericanEnglish, "Vietnam"},
		TagStr{language.German, "Vietnam"},
	},
	"VU": []TagStr{
		TagStr{language.AmericanEnglish, "Vanuatu"},
		TagStr{language.German, "Vanuatu"},
	},
	"WF": []TagStr{
		TagStr{language.AmericanEnglish, "Wallis and Futuna"},
		TagStr{language.German, "Wallis et Futuna"},
	},
	"WS": []TagStr{
		TagStr{language.AmericanEnglish, "Samoa"},
		TagStr{language.German, "Samoa"},
	},
	"YE": []TagStr{
		TagStr{language.AmericanEnglish, "Yemen"},
		TagStr{language.German, "Jemen"},
	},
	"YT": []TagStr{
		TagStr{language.AmericanEnglish, "Mayotte"},
		TagStr{language.German, "Mayotte"},
	},
	"ZA": []TagStr{
		TagStr{language.AmericanEnglish, "South Africa"},
		TagStr{language.German, "Südafrika"},
	},
	"ZM": []TagStr{
		TagStr{language.AmericanEnglish, "Zambia"},
		TagStr{language.German, "Sambia"},
	},
	"ZW": []TagStr{
		TagStr{language.AmericanEnglish, "Zimbabwe"},
		TagStr{language.German, "Zimbabwe"},
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

func (lang Language) TranslateAndSort(ids []string) []IDName {
	result := make([]IDName, len(ids))
	for i, id := range ids {
		result[i].ID = id
		result[i].Name = lang.Translate(id)
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
	for _, euCode := range EUCountryCodes {
		if euCode == s {
			return true
		}
	}
	return false
}
