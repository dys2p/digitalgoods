package digitalgoods

import "html/template"

type Variant struct {
	ID        string
	Name      string
	ImageLink string
	Price     int // euro cents
	OnDemand  bool
}

func (variant Variant) NameHTML() template.HTML {
	return template.HTML(variant.Name)
}
