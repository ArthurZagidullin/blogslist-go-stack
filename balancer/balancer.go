package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	// "github.com/streadway/amqp"
	"os"
	"time"
)
import _ "github.com/go-sql-driver/mysql"

type Configuration struct {
	Mysql    string `json:"mysql"`
	Rabbitmq string `json:"rabbitmq"`
	Port     string `json:"port"`
}

type Video struct {
	Id string
}

type Worker struct {
	Balancer string
	Cdn      string
	Port     string
	Limit    int
	Videos   []Video
}

func (w *Worker) AddVideo(v Video) (ok bool, err error) {
	res, err := http.Get(w.Cdn + "/add/" + v.Id)
	if err == nil {
		if res.StatusCode == http.StatusOK {
			w.Videos = append(w.Videos, v)
			return true, nil
		}
		return false, nil
	}
	return false, err
}

// var WorkerCollection []Worker

var Config Configuration

func (c *Configuration) Init() error {
	var filename string

	flag.StringVar(&filename, "c", "config/local-example.conf", "Configuration filename")
	flag.Parse()

	fmt.Printf("Config filename: %s \n", filename)

	if file, err := ioutil.ReadFile(filename); err == nil {
		err = json.Unmarshal(file, c)
	}

	return nil
}

type WorkerCortage struct {
	Workers []*Worker
}

var ws WorkerCortage

// var FreeWorkers []Worker

func (ws *WorkerCortage) Add(w *Worker) bool {
	for _, wk := range ws.Workers {
		if wk.Cdn == w.Cdn {
			return false
		}
	}
	ws.Workers = append(ws.Workers, w)
	return true
}

func (ws *WorkerCortage) Free() int {
	var res int
	for _, wk := range ws.Workers {
		if len(wk.Videos) < wk.Limit {
			res++
		}
	}
	return res
}

func GetVideos(db *sql.DB) []Video {
	var videos []Video

	rows, err := db.Query("SELECT video_id FROM video")
	if err != nil {
		panic(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var video_id string

		if err := rows.Scan(&video_id); err != nil {
			panic(err.Error())
		}
		videos = append(videos, Video{video_id})
	}
	return videos
}

func RegWorker(w http.ResponseWriter, r *http.Request) {
	var wk Worker
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&wk)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	fmt.Fprintf(w, "%#v added!\n", wk)
	w.WriteHeader(http.StatusOK)

	fmt.Printf("Worker: %v", wk)
	ws.Add(&wk)
}

func main() {
	if err := Config.Init(); err != nil {
		fmt.Printf("Config initialization FAILED! %s \n", err)
	}

	fmt.Printf("%v\n", Config)

	go func() {
		fmt.Fprintf(os.Stdout, "Port listen %s\n", Config.Port)
		http.HandleFunc("/reg-worker/", RegWorker)
		http.ListenAndServe(":"+Config.Port, nil)
	}()

	var DB *sql.DB
	var err error
	if DB, err = sql.Open("mysql", Config.Mysql); err != nil {
		fmt.Printf("Connection DB FAILED! %s \n", err)
	}
	defer DB.Close()

	fmt.Println("Connection success!")

	videos := GetVideos(DB)

	//Все видео на момент запуска
	//0 воркеров
	for {
		if len(videos) > 0 && ws.Free() > 0 {
			for i, v := range videos {
				fmt.Printf("video #%d - id: %s\n", i, v.Id)
				for _, w := range ws.Workers {
					fmt.Printf("worker #%s; Limit: %d; Video: %d \n", w.Cdn, w.Limit, len(w.Videos))

					if len(w.Videos) < w.Limit {
						if ok, _ := w.AddVideo(v); ok {
							//Удаляем видео из списка
							videos[i] = videos[len(videos)-1]
						}
					}
				}
				time.Sleep(5000 * time.Millisecond)
			}

		}
	}
}
