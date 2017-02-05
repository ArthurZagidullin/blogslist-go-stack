//Структуры воркера
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
)

type Configuration struct {
	Balancer string `json:"balancer"`
	Cdn      string `json:"cdn"`
	Port     string `json:"port"`
	ApiKey   string `json:"api_key"`
	Limit    int    `json:"limit"`
	Rabbitmq string `json:"rabbitmq"`
	Period   int    `json:"period"`
}

func (c *Configuration) Init() error {
	var filename string

	flag.StringVar(&filename, "c", "config/local-example.conf", "Configuration filename")
	flag.Parse()

	if file, err := ioutil.ReadFile(filename); err == nil {
		if err = json.Unmarshal(file, c); err != nil {
			return nil
		}
	}
	fmt.Printf("Config filename: %s - %#v \n", filename, c)

	return nil
}
