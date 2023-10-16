package digitalgoods

import (
	"html/template"

	"github.com/dys2p/eco/lang"
)

type Category struct {
	ID          string
	Name        string
	Description map[string]string
}

// only supports langs which exist as Description key, TODO: language.Matcher
func (c *Category) TranslateDescription(l lang.Lang) template.HTML {
	return template.HTML(c.Description[string(l)])
}
