package db

import (
	"html/template"

	"github.com/dys2p/digitalgoods/html"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Category struct {
	ID          string
	Name        string
	Description []html.TagStr
}

func (c *Category) Translate(lang html.Language) template.HTML {
	// choose language tag from list of translations
	langs := make([]language.Tag, len(c.Description))
	for i := range c.Description {
		langs[i] = c.Description[i].Tag
	}
	if len(langs) == 0 {
		return ""
	}
	tag, i := language.MatchStrings(language.NewMatcher(langs), string(lang))
	return template.HTML(message.NewPrinter(tag).Sprint(c.Description[i].Str))
}
