package main

import (
	"bufio"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/ericchiang/css"
	"github.com/gocarina/gocsv"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
)

func writer(input <- chan string, csvFile *os.File, wg *sync.WaitGroup, markAsRead bool) {

	titleSelector, err := css.Compile("h1.title")

	if err != nil {
		log.Fatal(err)
	}

	authorSelector, err := css.Compile("span.auts > a")

	if err != nil {
		log.Fatal(err)
	}

	authorSelector2, err := css.Compile("#authorname1")

	if err != nil {
		log.Fatal(err)
	}

	authorSelector3, err := css.Compile("span.auts")

	if err != nil {
		log.Fatal(err)
	}

	publisherSelector, err := css.Compile("p.pubinf")

	if err != nil {
		log.Fatal(err)
	}

	isbn13Selector, err := css.Compile("p.i13")

	if err != nil {
		log.Fatal(err)
	}

	isbn10Selector, err := css.Compile("p.i10")

	if err != nil {
		log.Fatal(err)
	}

	imageSelector, err := css.Compile("img.cover")

	if err != nil {
		log.Fatal(err)
	}

	yearRegexp, err := regexp.Compile("[^0-9]+")

	if err != nil {
		log.Fatal(err)
	}

	isbnRegexp, err := regexp.Compile(`.+\s+([0-9-]+)`)

	if err != nil {
		log.Fatal(err)
	}

	func () {

	for {
		select {
		case isbn, more := <-input:

			if more == true {

				isbn = strings.Replace(isbn, "-", "", -1)
				isbn = strings.Replace(isbn, " ", "", -1)

				// isbn = "978-83-7574-943-4"

				// isbn = "978-83-7964-368-4"

				req, _ := http.NewRequest("GET", "https://www.books-by-isbn.com/cgi-bin/isbn-lookup.pl?isbn="+isbn, nil)
				req.Header.Add("Connection", "keep-alive")
				req.Header.Add("Upgrade-Insecure-Requests", "1")
				req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36")
				req.Header.Add("Accept", "application/json,text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
				req.Header.Add("Referer", "https://www.books-by-isbn.com/")
				//req.Header.Add("Accept-Encoding", "gzip, deflate, br")
				req.Header.Add("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")

				res, err := http.DefaultClient.Do(req)

				if err != nil {
					log.Fatal(err)
				}

				node, err := html.Parse(res.Body)

				if err != nil {
					log.Fatal(err)
				}

				res.Body.Close()

				if res.StatusCode != 200 {
					log.Fatal("got non 200 response on gettign details")
				}

				b := &Book{}
				b.ID = isbn

				for _, ele := range titleSelector.Select(node) {
					b.Title = ele.FirstChild.Data
				}

				for _, ele := range authorSelector.Select(node) {
					b.Author = ele.FirstChild.Data
				}

				if b.Author == "" {
					for _, ele := range authorSelector2.Select(node) {
						b.Author = ele.FirstChild.Data
					}
				}

				if b.Author == "" {
					for _, ele := range authorSelector3.Select(node) {
						b.Author = ele.FirstChild.Data
					}
				}

				for _, ele := range isbn13Selector.Select(node) {
					b.ISBN13 = isbnRegexp.FindStringSubmatch(ele.FirstChild.Data)[1]
				}

				for _, ele := range isbn10Selector.Select(node) {
					b.ISBN10 = isbnRegexp.FindStringSubmatch(ele.FirstChild.Data)[1]
				}

				for _, ele := range publisherSelector.Select(node) {
					b.Publisher = ele.FirstChild.FirstChild.Data
					if ele.FirstChild.NextSibling != nil {
						b.Year = string(yearRegexp.ReplaceAll([]byte(ele.FirstChild.NextSibling.Data), []byte{}))
					}
				}

				for _, ele := range imageSelector.Select(node) {
					for _, att := range ele.Attr {
						if att.Key == "src" {
							b.Image = "https://www.books-by-isbn.com" + att.Val
						}
					}
				}

				if b.Title != "" && b.Author != "" && b.Title != "a" {
					b.Complete = true
				}

				b.Read = markAsRead

				b.Link = res.Request.URL.String()

				err = gocsv.MarshalWithoutHeaders([]*Book{b}, csvFile)

				if err != nil {
					log.Fatal(err)
				}
				spew.Dump(b)

				wg.Done()

			} else {
				return
			}
		}
	}

	}()

}

func startScanning(file string, markAsRead bool) {

	csvFile, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND, os.ModePerm)

	if err != nil {

		if os.IsNotExist(err) {
			csvFile, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)

			err = gocsv.MarshalFile([]*Book{}, csvFile)

			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}

	channel := make(chan string)

	wg := &sync.WaitGroup{}

	go writer(channel, csvFile, wg, markAsRead)

	for {

		if markAsRead {
			log.Println("Marking books as read.")
		}

		scanner := bufio.NewScanner(os.Stdin)

		fmt.Print("Enter ISBN: ")

		scanner.Scan()

		text := scanner.Text()

		if len(text) == 0 {
			wg.Wait()
			close(channel)
			return
		}

		wg.Add(1)

		channel <- text

		wg.Wait()

	}
}