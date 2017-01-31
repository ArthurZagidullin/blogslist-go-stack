package video

import (
	"time"
)

type Video struct {
	Id      string
	Created time.Time
	Updated time.Time
}

func (v *Video) Update() {

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
