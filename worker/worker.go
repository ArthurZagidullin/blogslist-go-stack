//Слушает порт из конфига
//обновляет видео по кругу
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	// "os"
	"time"
	"worker/config"
	"worker/video"
)

const (
	// See http://golang.org/pkg/time/#Parse
	timeFormat = "2006-01-02 15:04 MST"
)

var Configuration config.Configuration
var Stack video.Cortage

func Registration() bool {
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(Configuration)

	url := Configuration.Balancer + "/reg-worker/"

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		log.Panic(err)
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func handler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/add/"):]

	log.Printf("Id catch %s!\n", id)
	if len(Stack.Videos) < Configuration.Limit {
		if ok := Stack.Add(id); !ok {
			log.Printf("Id %s is exist in Cortage!\n", id)
			w.WriteHeader(http.StatusNotAcceptable)
		}

		log.Printf("Id %s Add in Cortage! Now len %d\n", id, len(Stack.Videos))
		w.WriteHeader(http.StatusOK)
	}
}

//На вход принимается ключ API
//период работы и список id-видео
//список может дополнятся
func main() {
	Configuration.Init()

	log.Println("Start Registration worker!")

	//Регистрация воркера
	for {
		log.Println("Trying to auth worker..")
		if Registration() {
			log.Println("Worker authorized!")
			break
		} else {
			time.Sleep(5000 * time.Millisecond)
		}
	}

	go func() {
		log.Printf("Port listen %s", Configuration.Port)
		http.HandleFunc("/add/", handler)
		http.ListenAndServe(":"+Configuration.Port, nil)
	}()

	itr := 0
	log.Println("Loop started!")
	for {
		if len(Stack.Videos) > 0 {
			log.Printf("Itteration #%d \n", itr)
			period := time.Duration(5000 / len(Stack.Videos))
			for _, vid := range Stack.Videos {
				log.Printf("- Video update, id %s\n", vid.Id)
				time.Sleep(period * time.Millisecond)
			}
			itr++
		}
	}
}
