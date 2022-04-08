package digitalgoods

import (
	"golang.org/x/text/language"
)

type Category struct {
	ID               string
	Name             string
	DescriptionLangs []language.Tag
	DescriptionTexts []string // same index as DescriptionLangs
}
