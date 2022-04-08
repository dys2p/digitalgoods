package digitalgoods

// ISO-3166-1
var ISOCountryCodes = [...]string{
	"AF", "AX", "AL", "DZ", "AS", "AD", "AO", "AI", "AQ", "AG", "AR", "AM", "AW", "AU", "AT", "AZ", "BS", "BH", "BD", "BB", "BY", "BE", "BZ", "BJ", "BM", "BT", "BO", "BQ", "BA", "BW", "BV", "BR", "IO", "BN", "BG", "BF", "BI", "KH", "CM", "CA", "CV", "KY", "CF", "TD", "CL", "CN", "CX", "CC", "CO", "KM", "CG", "CD", "CK", "CR", "CI", "HR", "CU", "CW", "CY", "CZ", "DK", "DJ", "DM", "DO", "EC", "EG", "SV", "GQ", "ER", "EE", "ET", "FK", "FO", "FJ", "FI", "FR", "GF", "PF", "TF", "GA", "GM", "GE", "DE", "GH", "GI", "GR", "GL", "GD", "GP", "GU", "GT", "GG", "GN", "GW", "GY", "HT", "HM", "VA", "HN", "HK", "HU", "IS", "IN", "ID", "IR", "IQ", "IE", "IM", "IL", "IT", "JM", "JP", "JE", "JO", "KZ", "KE", "KI", "KP", "KR", "KW", "KG", "LA", "LV", "LB", "LS", "LR", "LY", "LI", "LT", "LU", "MO", "MK", "MG", "MW", "MY", "MV", "ML", "MT", "MH", "MQ", "MR", "MU", "YT", "MX", "FM", "MD", "MC", "MN", "ME", "MS", "MA", "MZ", "MM", "NA", "NR", "NP", "NL", "NC", "NZ", "NI", "NE", "NG", "NU", "NF", "MP", "NO", "OM", "PK", "PW", "PS", "PA", "PG", "PY", "PE", "PH", "PN", "PL", "PT", "PR", "QA", "RE", "RO", "RU", "RW", "BL", "SH", "KN", "LC", "MF", "PM", "VC", "WS", "SM", "ST", "SA", "SN", "RS", "SC", "SL", "SG", "SX", "SK", "SI", "SB", "SO", "ZA", "GS", "SS", "ES", "LK", "SD", "SR", "SJ", "SZ", "SE", "CH", "SY", "TW", "TJ", "TZ", "TH", "TL", "TG", "TK", "TO", "TT", "TN", "TR", "TM", "TC", "TV", "UG", "UA", "AE", "GB", "US", "UM", "UY", "UZ", "VU", "VE", "VN", "VG", "VI", "WF", "EH", "YE", "ZM", "ZW"}

func IsISOCountryCode(s string) bool {
	for _, code := range ISOCountryCodes {
		if code == s {
			return true
		}
	}
	return false
}

type Article struct {
	ID         string
	CategoryID string
	Name       string
	Price      int            // euro cents
	Stock      map[string]int // key is country or "all"
	OnDemand   bool
	Hide       bool // see Portfolio() for precedence
	HasCountry bool // ISO country
}

// Max returns the max value which can be ordered (like the HTML input max attribute). The stock quantity should be displayed separately, so users know how many items can be delivered instantly.
func (a Article) Max(countryID string) int {
	max := a.Stock[countryID]
	if a.OnDemand {
		max += 100
	}
	return max
}

func (a Article) OnDemandOnly(countryID string) bool {
	return a.Stock[countryID] == 0 && a.OnDemand
}

// Portfolio determines whether an article is shown in the portfolio. It might be still sold out at the moment.
func (a Article) Portfolio() bool {
	if len(a.Stock) > 0 || a.OnDemand {
		return true
	}
	return !a.Hide
}

func (a *Article) FeaturedCountryIDs() []string {
	if !a.HasCountry {
		return []string{"all"}
	}
	ids := []string{}
	for _, countryID := range ISOCountryCodes {
		if stock := a.Stock[countryID]; stock > 0 {
			ids = append(ids, countryID)
		}
	}
	return ids
}

func (a *Article) OtherCountryIDs() []string {
	if !a.HasCountry {
		return nil
	}
	if !a.OnDemand {
		return nil
	}
	ids := []string{}
	for _, countryID := range ISOCountryCodes {
		if stock := a.Stock[countryID]; stock == 0 {
			ids = append(ids, countryID)
		}
	}
	return ids
}
