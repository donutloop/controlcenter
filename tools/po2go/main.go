package main

import (
	"flag"
	"fmt"
	"github.com/robfig/gettext-go/gettext/po"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var (
	poBase  = flag.String("i", "", "po files base dir")
	outfile = flag.String("o", "", "output filename")
)

func main() {
	flag.Parse()

	outFile, err := os.Create(*outfile)

	if err != nil {
		panic(err)
	}

	defer func() {
		if err := outFile.Close(); err != nil {
			panic(err)
		}
	}()

	entries, err := ioutil.ReadDir(*poBase)

	if err != nil {
		panic(err)
	}

	filenamesByLanguage := map[string][]string{}

	for _, e := range entries {
		messagesDir := path.Join(*poBase, e.Name(), "LC_MESSAGES")

		s, err := os.Stat(messagesDir)

		if err != nil || !s.IsDir() {
			continue
		}

		langname := path.Base(e.Name())

		_, exists := filenamesByLanguage[langname]

		if !exists {
			filenamesByLanguage[langname] = []string{}
		}

		entries, err := ioutil.ReadDir(messagesDir)

		if err != nil {
			panic(err)
		}

		for _, e := range entries {
			filename := path.Join(messagesDir, e.Name())
			filenamesByLanguage[langname] = append(filenamesByLanguage[langname], filename)
		}
	}

	fmt.Fprintf(outFile, `// Code generated by running "go generate". DO NOT EDIT.
package po

import (
	"golang.org/x/text/language"
)

func init() {`)

	for lang, filenames := range filenamesByLanguage {
		fmt.Fprintln(outFile, `
	{
		lang := language.MustParse("`+lang+`")
	`)

		for _, filename := range filenames {
			f, err := po.Load(filename)

			if err != nil {
				panic(err)
			}

			for _, msg := range f.Messages {
				// skip messages with no translation
				if len(strings.TrimSpace(msg.MsgStr)) > 0 {
					fmt.Fprintf(outFile, "\n\t\tDefaultCatalog.SetString(lang, `%s`, `%s`);", msg.MsgId, msg.MsgStr)
				}
			}
		}

		fmt.Fprintln(outFile, "\t}")
	}

	fmt.Fprintf(outFile, "\n}")
}
