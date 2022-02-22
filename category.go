package digitalgoods

import (
	"html/template"

	"github.com/dys2p/digitalgoods/html"
)

type Category struct {
	ID          string
	Name        string
	Description []html.TagStr
}

func (c *Category) Translate(lang html.Language) template.HTML {
	return template.HTML(lang.TranslateItem(c.Description))
}
