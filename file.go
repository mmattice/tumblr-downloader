package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"time"
)

var (
	gfyRequest = "https://gfycat.com/cajax/get/%s"
)

// A File contains information on a particular tumblr URL, as well as the user where the URL was found.
type File struct {
	User          *User
	URL           string
	UnixTimestamp int64
	Filename      string
}

func newFile(URL string) File {
	return File{
		URL:      URL,
		Filename: path.Base(URL),
	}
}

// Download downloads a file specified in the file's URL.
func (f File) Download() {
	filepath := path.Join(cfg.DownloadDirectory, f.User.String(), path.Base(f.Filename))
	var resp *http.Response
	var err error
	var pic []byte

	for {
		resp, err = http.Get(f.URL)
		if err != nil {
			log.Println(err)
			continue
		}
		defer resp.Body.Close()

		pic, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("ReadAll:", err)
			continue
		}

		break
	}

	err = ioutil.WriteFile(filepath, pic, 0644)
	if err != nil {
		log.Fatal("WriteFile:", err)
	}

	err = os.Chtimes(filepath, time.Now(), time.Unix(f.UnixTimestamp, 0))
	if err != nil {
		log.Println(err)
	}

	FileTracker.Signal(f.Filename)

	pBar.Increment()
	f.User.downloadWg.Done()
	atomic.AddUint64(&f.User.filesProcessed, 1)
	atomic.AddUint64(&gStats.filesDownloaded, 1)
	atomic.AddUint64(&gStats.bytesDownloaded, uint64(len(pic)))

}

// String is the standard method for the Stringer interface.
func (f File) String() string {
	date := time.Unix(f.UnixTimestamp, 0)
	return f.User.String() + " - " + date.Format("2006-01-02 15:04:05") + " - " + path.Base(f.Filename)
}

// Gfy houses the Gfycat response.
type Gfy struct {
	GfyItem struct {
		Mp4Url  string `json:"mp4Url"`
		WebmURL string `json:"webmUrl"`
	} `json:"gfyItem"`
}

// GetGfycatURL gets the appropriate Gfycat URL for download, from a "normal" link.
func GetGfycatURL(slug string) string {
	gfyURL := fmt.Sprintf(gfyRequest, slug)

	var resp *http.Response
	for {
		resp2, err := http.Get(gfyURL)
		if err != nil {
			log.Println("GetGfycatURL: ", err)
		} else {
			resp = resp2
			break
		}
	}
	defer resp.Body.Close()

	gfyData, err := ioutil.ReadAll(resp.Body)
	checkFatalError(err)

	var ret string;
	if string(gfyData) != "Not Found" {
		var gfy Gfy

		err = json.Unmarshal(gfyData, &gfy)
		checkFatalError(err, "Gfycat Unmarshal:", string(gfyData))
		ret = gfy.GfyItem.Mp4Url
	} else {
		ret = ""
	}

	return ret
}

func getGfycatFiles(b, slug string) []File {
	var files []File
	regexResult := gfycatSearch.FindStringSubmatch(b)
	if regexResult != nil {
		for i, v := range regexResult[1:] {
			var url = GetGfycatURL(v)
			if url != "" {
				gfyFile := newFile(url)
				if slug != "" {
					gfyFile.Filename = fmt.Sprintf("%s_gfycat_%02d.mp4", slug, i+1)
				}
				files = append(files, gfyFile)
			}
		}
	}
	return files
}
