package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	// "os"
	"time"
)
import _ "github.com/go-sql-driver/mysql"

type Configuration struct {
	Mysql    string `json:"mysql"`
	Rabbitmq string `json:"rabbitmq"`
	Port     string `json:"port"`
}

type Video struct {
	Id      string
	VideoId int
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

	log.Printf("Config filename: %s \n", filename)

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

	rows, err := db.Query("SELECT id, video_id FROM video")
	if err != nil {
		panic(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		var video_id string
		var id int

		if err := rows.Scan(&id, &video_id); err != nil {
			panic(err.Error())
		}
		videos = append(videos, Video{Id: video_id, VideoId: id})
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

	log.Printf("Worker: %v", wk)
	ws.Add(&wk)
}

func main() {
	numCPU := runtime.NumCPU()
	log.Printf("GOMAXPROCS = %d\n", numCPU)
	runtime.GOMAXPROCS(numCPU * 2)

	if err := Config.Init(); err != nil {
		log.Printf("Config initialization FAILED! %s \n", err)
	}

	log.Printf("%v\n", Config)

	go func() {
		log.Printf("Port listen %s\n", Config.Port)
		http.HandleFunc("/reg-worker/", RegWorker)
		http.ListenAndServe(":"+Config.Port, nil)
	}()

	var DB *sql.DB
	var err error
	if DB, err = sql.Open("mysql", Config.Mysql); err != nil {
		log.Printf("Connection DB FAILED! %s \n", err)
	}
	defer DB.Close()

	// insertStmt, err := DB.Prepare("INSERT video_statistic SET `video_id`=?,`view`=?,`like`=?")
	insertStmt, err := DB.Prepare("INSERT video_statistic SET `video_id`=?,`view`=?,`like`=?,`dislike`=?,`favorite`=?,`comment`=?,`datetime`=?")
	if err != nil {
		panic("Stmt prepare error: " + err.Error())
	}

	//Подключение к брокеру очередей
	rabbitConn, err := amqp.Dial(Config.Rabbitmq)
	if err != nil {
		panic("Rabbitmq connection: " + err.Error())
	}
	defer rabbitConn.Close()

	statChannel, err := rabbitConn.Channel()
	if err != nil {
		panic(err.Error())
	}
	defer statChannel.Close()

	log.Println("Connection success!")

	videos := GetVideos(DB)

	go func(c *amqp.Channel, insertStmt *sql.Stmt) {
		durable, exclusive := false, false
		autoDelete, noWait := true, true

		q, _ := c.QueueDeclare("logs", durable, autoDelete, exclusive, noWait, nil)
		c.QueueBind(q.Name, "#", "logs", false, nil)

		autoAck, exclusive, noLocal, noWait := false, false, false, false

		messages, _ := c.Consume(q.Name, "logs", autoAck, exclusive, noLocal, noWait, nil)
		multiAck := false

		for msg := range messages {
			log.Println("Body:", string(msg.Body), "Timestamp:", msg.Timestamp)
			msg.Ack(multiAck)

			var s struct {
				Id, ViewCount, LikeCount, DislikeCount, FavorCount, CommentCount string
			}

			err := json.Unmarshal(msg.Body, &s)
			if err != nil {
				log.Println("Unmarshal msg body error: " + err.Error())
			}

			rows, err := DB.Query("SELECT id FROM video WHERE video_id='" + s.Id + "'")
			if err != nil {
				log.Println("Select video Error " + err.Error())
			}
			defer rows.Close()

			var id int

			for rows.Next() {
				rows.Scan(&id)
			}

			if id > 0 {
				_, err := insertStmt.Exec(id, s.ViewCount, s.LikeCount, s.DislikeCount, s.FavorCount, s.CommentCount, time.Now())
				if err != nil {
					log.Println("Statistic insert error: " + err.Error())
				}
				log.Printf("Insert statistic %s\n", s.Id)
			}
		}

	}(statChannel, insertStmt)

	log.Printf("Video %d fined!\n", len(videos))

	//Все видео на момент запуска
	//0 воркеров
	runtime.LockOSThread()
	for {
		if len(videos) > 0 && ws.Free() > 0 {
			for i, v := range videos {
				log.Printf("video #%d - id: %s\n", i, v.Id)
				for _, w := range ws.Workers {
					log.Printf("worker #%s; Limit: %d; Video: %d \n", w.Cdn, w.Limit, len(w.Videos))

					if len(w.Videos) < w.Limit {
						if ok, _ := w.AddVideo(v); ok {
							//Удаляем видео из списка
							videos[i] = videos[len(videos)-1]
						}
					}
				}
				time.Sleep(5000 * time.Millisecond)
			}
		} else {
			runtime.Gosched()
			log.Println("Zero free workers!")
			time.Sleep(5000 * time.Millisecond)
		}
	}
}
