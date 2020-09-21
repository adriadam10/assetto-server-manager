package views

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type TemplateLoader struct {
	pages, partials []string
}

func (t *TemplateLoader) Init() error {
	for fname, data := range _escData {
		if data.IsDir() {
			continue
		}

		if strings.HasPrefix(fname, "/pages/") {
			t.pages = append(t.pages, fname)
		} else if strings.HasPrefix(fname, "/partials/") {
			t.partials = append(t.partials, fname)
		}
	}

	return nil
}

func (t *TemplateLoader) fileContents(name string) (string, error) {
	f, err := _escStatic.Open(name)

	if err != nil {
		return "", err
	}

	defer f.Close()

	buf := new(bytes.Buffer)

	_, err = io.Copy(buf, f)

	if err != nil {
		return "", nil
	}

	return buf.String(), nil
}

func (t *TemplateLoader) Templates(funcs template.FuncMap) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	templateData, err := t.fileContents("/layout/base.html")

	if err != nil {
		return nil, err
	}

	for _, partial := range t.partials {
		contents, err := t.fileContents(partial)

		if err != nil {
			return nil, err
		}

		templateData += contents
	}

	errGroup, _ := errgroup.WithContext(context.Background())
	var mutex sync.Mutex

	for _, page := range t.pages {
		page := page

		errGroup.Go(func() error {
			pageData := templateData

			pageText, err := t.fileContents(page)

			if err != nil {
				return err
			}

			pageData += pageText

			t, err := template.New(page).Funcs(funcs).Parse(pageData)

			if err != nil {
				return err
			}

			mutex.Lock()
			templates[strings.TrimPrefix(filepath.ToSlash(page), "/pages/")] = t
			mutex.Unlock()

			return nil
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	return templates, nil
}
