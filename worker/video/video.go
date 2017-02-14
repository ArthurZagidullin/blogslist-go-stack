package video

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Video struct {
	Id      string
	Created time.Time
	Updated time.Time
}

func (v *Video) Update(apiKey string) Statistic {
	log.Printf("- Video update, id %s\n", v.Id)
	url := "https://www.googleapis.com/youtube/v3/videos?id=" + v.Id + "&part=statistics&key=" + apiKey
	res, _ := http.Get(url)
	jsonBody, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	// log.Printf("%s", robots)
	var stat Stat
	err = json.Unmarshal(jsonBody, &stat)
	if err != nil {
		log.Println("error:", err)
	}
	log.Printf("%+v", stat)
	var result Statistic
	if len(stat.Items) > 0 {
		result = stat.Items[0].Statistics
		result.Id = v.Id
	}
	return result
}

type Stat struct {
	Items []Item `json:"items"`
}

type Item struct {
	Statistics Statistic `json:"statistics"`
}

type Statistic struct {
	Id, ViewCount, LikeCount, DislikeCount, FavorCount, CommentCount string
}

type Cortage struct {
	Videos []Video
}

func (c *Cortage) Add(id string) bool {
	for _, vid := range c.Videos {
		if vid.Id == id {
			return false
		}
	}
	c.Videos = append(c.Videos, Video{id, time.Now(), time.Now()})
	return true
}
