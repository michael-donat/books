package main

import (
	"bytes"
	"github.com/gobuffalo/packr/v2"
	"html/template"
	"log"
	"net/http"
)

func startServer(file string) {

	box := packr.New("TPL", ".")

	htmlSource, err := box.FindString("template.html")

	if err != nil {
		log.Fatal(err)
	}

	tpl := template.Must(template.New("index").Parse(htmlSource))


	http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {

		books := loadBooks(file)
		content := &bytes.Buffer{}
		err = tpl.Execute(content, books)

		if err != nil {
			log.Fatal(err)
		}

		w.Header().Set("Content-type", "text/html")
		w.Write([]byte(content.String()))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}