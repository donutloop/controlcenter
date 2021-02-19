// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package translator

import (
	"fmt"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
	"strings"
)

// Allows extractions of language keys
func I18n(s string) string {
	return s
}

type TranslatableStringer interface {
	TplString() string
	Args() []interface{}
}

// Given a string supported by gettext, transform it into something consumable by go-text
// NOTE: right now what we are looking for it just to prevent go-text of interpreting %
// by duplicating it, meaning "literal percent".
// TODO: this function will need to be smarter in order to support positional
// arguments and pluralization. Please see gitlab issue #245 for more info.
func TransformTranslation(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

func Stringfy(s TranslatableStringer) string {
	return fmt.Sprintf(s.TplString(), s.Args()...)
}

type Translator interface {
	Translate(string, ...interface{}) (string, error)
}

type Translators interface {
	Translator(language.Tag) Translator
	Matcher() language.Matcher
}

type translators struct {
	catalog catalog.Catalog
}

func (t *translators) Translator(tag language.Tag) Translator {
	return newTranslator(tag, t.catalog)
}

func (t *translators) Matcher() language.Matcher {
	return t.catalog.Matcher()
}

func New(catalog catalog.Catalog) Translators {
	return &translators{catalog: catalog}
}

type translator struct {
	printer *message.Printer
}

func newTranslator(tag language.Tag, c catalog.Catalog) *translator {
	return &translator{printer: message.NewPrinter(tag, message.Catalog(c))}
}

func (t *translator) Translate(s string, args ...interface{}) (string, error) {
	return t.printer.Sprintf(message.Key(s, s), args), nil
}

func Translate(t Translator, c TranslatableStringer) (string, error) {
	transformed := TransformTranslation(c.TplString())

	translated, err := t.Translate(transformed)
	if err != nil {
		return "", errorutil.Wrap(err)
	}

	args := c.Args()

	// TODO: restore this, or better, rely on the translator!
	// for i, arg := range args {
	// 	t, ok := arg.(time.Time)
	// 	if ok {
	// 		args[i] = timeutil.PrettyFormatTime(t, language)
	// 	}
	// }

	return fmt.Sprintf(translated, args...), nil
}
