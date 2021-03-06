package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gobuffalo/packr/v2"
	"github.com/gocarina/gocsv"
	"github.com/urfave/cli"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

func dedupe(file string) {
	state := map[string]bool{}
	books := loadBooks(file)

	deduped := []*Book{}

	for _, book := range books {
		complete, existing := state[book.ID]

		if (complete && existing) || (existing && book.Complete == false) {
			continue
		}

		if existing && book.Complete {
			// replace
			for idx, override := range deduped {
				if override.ID == book.ID {
					deduped[idx] = book
				}
			}
			continue
		}

		deduped = append(deduped, book)
		state[book.ID] = book.Complete

	}

	csvFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)

	if err != nil {
		log.Fatal(err)
	}

	defer csvFile.Close()

	err = gocsv.MarshalFile(deduped, csvFile)

	if err != nil {
		log.Fatal(err)
	}

}

func formatISBN(s string) string {

	var buffer bytes.Buffer
	ss := []rune(s)

	if len(ss) == 13 {

		buffer.WriteRune(ss[0])
		buffer.WriteRune(ss[1])
		buffer.WriteRune(ss[2])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[3])
		buffer.WriteRune(ss[4])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[5])
		buffer.WriteRune(ss[6])
		buffer.WriteRune(ss[7])
		buffer.WriteRune(ss[8])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[9])
		buffer.WriteRune(ss[10])
		buffer.WriteRune(ss[11])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[12])

	} else if len(ss) == 10 {
		buffer.WriteRune(ss[0])
		buffer.WriteRune(ss[1])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[2])
		buffer.WriteRune(ss[3])
		buffer.WriteRune(ss[4])
		buffer.WriteRune(ss[5])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[6])
		buffer.WriteRune(ss[7])
		buffer.WriteRune(ss[8])
		buffer.WriteRune('-')
		buffer.WriteRune(ss[9])
	} else {
		return s
	}

	return buffer.String()
}

type Book struct {
	ID        string `url:"entry.72340741"`
	ISBN13    string `url:"entry.1363043213"`
	ISBN10    string `url:"entry.1321459061"`
	Title     string `url:"entry.2060886008"`
	Author    string `url:"entry.788898215"`
	Publisher string `url:"entry.923625951"`
	Year      string `url:"entry.1894089560"`
	Link      string `url:"entry.1895009924"`
	Image     string `url:"entry.2071078398"`
	Complete  bool
	Read bool
}

func (b *Book) ISBN() string {
	return formatISBN(b.ID)
}

func loadBooks(file string) []*Book {
	csvFile, err := os.OpenFile(file, os.O_RDONLY, os.ModePerm)

	if err != nil {
		log.Fatal(err)
	}

	defer csvFile.Close()

	books := []*Book{}

	if err := gocsv.UnmarshalFile(csvFile, &books); err != nil { // Load clients from file
		log.Fatal(err)
	}

	return books
}

func main() {

	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "data-file, f",
			Value: "books.csv",
		},
	}

	app.Commands = []cli.Command{
		{
			Name: "output",
			Action: func(ctx *cli.Context) error {

				box := packr.New("TPL", ".")

				htmlSource, err := box.FindString("template.html")

				if err != nil {
					log.Fatal(err)
				}

				tpl := template.Must(template.New("index").Parse(htmlSource))

				books := loadBooks(ctx.GlobalString("data-file"))
				content := &bytes.Buffer{}
				err = tpl.Execute(content, books)

				if err != nil {
					log.Fatal(err)
				}

				err = ioutil.WriteFile("index.html", content.Bytes(), 0755)

				if err != nil {
					log.Fatal(err)
				}

				return nil
			},
		},
		{
			Name: "serve",
			Action: func(ctx *cli.Context) error {
				startServer(ctx.GlobalString("data-file"))
				return nil
			},
		},
		{
			Name: "scan",
			Flags: []cli.Flag{
				cli.BoolTFlag{
					Name: "mark-as-read",
				},
			},
			Action: func(ctx *cli.Context) error {
				startScanning(ctx.GlobalString("data-file"), ctx.BoolT("mark-as-read"))
				return nil
			},
		},
		{
			Name: "scan-list",
			Action: func(ctx *cli.Context) error {
				books := loadBooks(ctx.GlobalString("data-file"))

				booksByID := map[string]*Book{}

				booksOutput := []*Book{}

				for _, book := range books {
					booksByID[book.ID] = book
				}

				scanner := bufio.NewScanner(os.Stdin)

				for {

					fmt.Print("Enter ISBN: ")

					scanner.Scan()

					text := scanner.Text()

					if len(text) == 0 {

						for _, o := range booksOutput {
							fmt.Printf("%s - %s\n", o.Author, o.Title)
						}

						return nil
					}

					text = strings.Replace(text, "-", "", -1)
					text = strings.Replace(text, " ", "", -1)

					if book, found := booksByID[text]; found {
						booksOutput = append(booksOutput, book)
					} else {
						log.Println("Book not found in list")
					}
				}

				return nil
			},
		},
		{
			Name: "complete",
			Action: func(ctx *cli.Context) error {

				books := loadBooks(ctx.GlobalString("data-file"))

				csvFile, err := os.OpenFile(ctx.GlobalString("data-file"), os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)

				if err != nil {
					log.Fatal(err)
				}

				channel := make(chan bookcheck)

				wg := &sync.WaitGroup{}

				go writer(channel, csvFile, wg)

				for _, book := range books {
					if book.Complete == true {
						continue
					}

					wg.Add(1)

					channel <- bookcheck{title: book.ID, read: book.Read}
				}

				wg.Wait()

				close(channel)

				csvFile.Close()

				dedupe(ctx.GlobalString("data-file"))

				return nil
			},
		},
		{
			Name: "dedup",
			Action: func(ctx *cli.Context) error {
				dedupe(ctx.GlobalString("data-file"))
				return nil
			},
		},
	}

	err := app.Run(os.Args)

	if err != nil {
		log.Fatal(err)
	}

}
