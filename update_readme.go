package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mmcdole/gofeed"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
)

func buildProgressBar(percent float64) string {
	bar := ""
	for i := 0; i < 25; i++ {
		if percent >= 4 {
			bar += "█"
		} else if percent >= 2 {
			bar += "▓"
		} else if percent >= 0 {
			bar += "▒"
		} else {
			bar += "░"
		}
		percent -= 4
	}
	return bar
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fetchWakatime() (string, error) {
	wakatimeToken := os.Getenv("WAKATIME_TOKEN")
	if wakatimeToken == "" {
		return "", errors.New("should provide WAKATIME_TOKEN")
	}
	res, err := http.Get(
		fmt.Sprintf("https://wakatime.com/api/v1/users/current/stats/last_7_days?api_key=%s", wakatimeToken),
	)
	if err != nil {
		return "", err
	}
	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	body := struct {
		Data struct {
			Languages []struct {
				Digital      string  `json:"digital"`
				Hours        int     `json:"languages"`
				Minutes      int     `json:"minutes"`
				Name         string  `json:"name"`
				Percent      float64 `json:"percent"`
				Text         string  `json:"text"`
				TotalSeconds float64 `json:"total_seconds"`
			} `json:"languages"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", err
	}
	section := ""
	langs := body.Data.Languages
	if len(langs) > 10 {
		langs = langs[:10]
	}
	maxNameLen, maxTextLen := 0, 0
	for _, lang := range langs {
		maxNameLen = max(len(lang.Name), maxNameLen)
		maxTextLen = max(len(lang.Text), maxTextLen)
	}

	for _, lang := range langs {
		pattern:=fmt.Sprintf("%%-%ds %%-%ds %%-25s %%.2f%%%%\n",maxNameLen,maxTextLen)
		section += fmt.Sprintf(pattern, lang.Name, lang.Text, buildProgressBar(lang.Percent), lang.Percent)
	}
	return fmt.Sprintf("```text\n%s```", section), nil
}

func fetchDouban() (string, error) {
	const doubanUrl = "https://www.douban.com/feed/people/sorcererxw/interests"

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(doubanUrl)
	if err != nil {
		return "", err
	}
	section := ""
	for _, it := range feed.Items {
		section += fmt.Sprintf("* <a href='%s' target='_blank'>%s</a>", it.Link, it.Title)
		if it.PublishedParsed != nil {
			section += fmt.Sprintf(" - <code>%s</code>", it.PublishedParsed.Format("2006/01/02"))
		}
		section += "\n"
	}
	return section, nil
}

var mux sync.RWMutex

func writeSection(section string, newContent string) error {
	file := "./README.md"
	mux.Lock()
	defer mux.Unlock()
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	content := string(raw)
	pattern := fmt.Sprintf("<!--START_SECTION:%s-->[\\S\\s]*\\s<!--END_SECTION:%s-->", section, section)
	log.Println(pattern)
	re := regexp.MustCompile(pattern)
	log.Println(re.MatchString(content))
	content = re.ReplaceAllString(
		content,
		fmt.Sprintf("<!--START_SECTION:%s-->\n%s\n<!--END_SECTION:%s-->", section, newContent, section),
	)
	if err := ioutil.WriteFile(file, []byte(content), os.ModeExclusive); err != nil {
		return err
	}
	return nil
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wakatimeSection, err := fetchWakatime()
		if err != nil {
			panic(err)
		}
		log.Print(wakatimeSection)
		if err := writeSection("waka", wakatimeSection); err != nil {
			panic(err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		doubanSection, err := fetchDouban()
		if err != nil {
			panic(err)
		}
		log.Print(doubanSection)
		if err := writeSection("douban", doubanSection); err != nil {
			panic(err)
		}
	}()
	wg.Wait()
}
