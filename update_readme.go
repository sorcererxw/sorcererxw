package main

import (
	"fmt"
	"github.com/mmcdole/gofeed"
	"log"
)

func generateDoubanSection() (string, error) {
	const doubanUrl = "https://www.douban.com/feed/people/sorcererxw/interests"

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(doubanUrl)
	if err != nil {
		return "", err
	}
	section := ""
	for _, it := range feed.Items {
		section += fmt.Sprintf("<a href='%s'>%s</a>", it.Link, it.Title)
		if it.PublishedParsed != nil {
			section += fmt.Sprintf("<code>%s</code>", it.PublishedParsed.Format("2006/01/02"))
		}
		section += "<br/>"
	}
	return section, nil
}

func main() {
	doubanSection, err := generateDoubanSection()
	if err != nil {
		panic(err)
	}
	log.Print(doubanSection)
}
