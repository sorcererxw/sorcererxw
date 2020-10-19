package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	URL "net/url"
	"os"
	"regexp"
	"sync"

	"github.com/mmcdole/gofeed"
)

func buildProgressBar(percent float64, span int) string {
	bar := ""
	for i := 0; i < 100/span; i++ {
		if percent >= float64(span) {
			bar += "█"
		} else if percent >= float64(span)/2 {
			bar += "▓"
		} else if percent >= 0 {
			bar += "▒"
		} else {
			bar += "░"
		}
		percent -= float64(span)
	}
	return bar
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type POI struct {
	Lon float64
	Lat float64
}

func fetchMap(geos []POI) error {
	mapboxToken := os.Getenv("MAPBOX_TOKEN")
	if mapboxToken == "" {
		return errors.New("should provice MAPBOX_TOKEN")
	}
	markers := ""
	for idx, geo := range geos {
		if idx == 0 {
			markers += "/"
		} else {
			markers += ","
		}
		markers += fmt.Sprintf("url-%s(%f,%f)", URL.PathEscape("https://cdn.jellow.site/spot.png"), geo.Lon, geo.Lat)
	}
	url := fmt.Sprintf("https://api.mapbox.com/styles/v1/mapbox/dark-v10/static%s/auto/1280x600@2x?logo=false&access_token=%s", markers, mapboxToken)
	oldHash, err := ioutil.ReadFile("./footprint.hash")
	if err != nil {
		return err
	}
	newHash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	if string(oldHash) == newHash {
		log.Println("footprint does not change")
		return nil
	}
	defer ioutil.WriteFile("./footprint.hash", []byte(newHash), os.ModePerm)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	file, err := os.OpenFile("./footprint.png", os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, res.Body)
	if err != nil {
		return err
	}
	return nil
}

func fetchFootprint() error {
	jikeUsername := os.Getenv("JIKE_USERNAME")
	if jikeUsername == "" {
		return errors.New("should provice JIKE_USERNAME")
	}
	url := fmt.Sprintf("https://api.ruguoapp.com/1.0/footprint-service/footprints/show?username=%s", jikeUsername)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	body := struct {
		Data struct {
			FeatureCollection struct {
				Type     string `json:"type"`
				Features []struct {
					Type     string `json:"type"`
					Geometry struct {
						Type        string    `json:"type"`
						Coordinates []float64 `json:"coordinates"`
					} `json:"geometry"`
					Properties struct {
						Count int `json:"count"`
					} `json:"properties"`
				} `json:"features"`
			} `json:"featureCollection"`
			Center struct {
				Type       string `json:"type"`
				Properties struct {
				} `json:"properties"`
				Geometry struct {
					Type        string    `json:"type"`
					Coordinates []float64 `json:"coordinates"`
				} `json:"geometry"`
			} `json:"center"`
			Envelope struct {
				Type       string `json:"type"`
				Properties struct {
				} `json:"properties"`
				Geometry struct {
					Type        string        `json:"type"`
					Coordinates [][][]float64 `json:"coordinates"`
				} `json:"geometry"`
			} `json:"envelope"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return err
	}
	pois := make([]POI, 0)
	for _, f := range body.Data.FeatureCollection.Features {
		pois = append(pois, POI{
			Lon: f.Geometry.Coordinates[0],
			Lat: f.Geometry.Coordinates[1],
		})
	}
	return fetchMap(pois)
}

func fetchWakatime() (string, error) {
	wakatimeToken := os.Getenv("WAKATIME_TOKEN")
	if wakatimeToken == "" {
		return "", errors.New("should provide WAKATIME_TOKEN")
	}
	res, err := http.Get(
		fmt.Sprintf("https://wakatime.com/api/v1/users/current/stats/last_30_days?api_key=%s", wakatimeToken),
	)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
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
	progressBarSpan := 10
	for _, lang := range langs {
		maxNameLen = max(len(lang.Name), maxNameLen)
		maxTextLen = max(len(lang.Text), maxTextLen)
	}

	for _, lang := range langs {
		pattern := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%.2f%%%%\n", maxNameLen, maxTextLen, progressBarSpan)
		section += fmt.Sprintf(pattern, lang.Name, lang.Text, buildProgressBar(lang.Percent, progressBarSpan), lang.Percent)
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
		section += fmt.Sprintf("* [%s](%s)", it.Title, it.Link)
		if it.PublishedParsed != nil {
			section += fmt.Sprintf(" <code>%s</code>", it.PublishedParsed.Format("2006/01/02"))
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
	re := regexp.MustCompile(pattern)
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := fetchFootprint(); err != nil {
			panic(err)
		}
	}()
	wg.Wait()
}
