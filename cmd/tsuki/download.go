package main

import (
	"fmt"
	"encoding/json"
)
type chunk struct {
	ChunkID string 	`json:"chunkID"`
	StorageIP string `json:"storageIP`
}
type downloadRequestAns struct {
	Status string	`json:"status"`
	Message string	`json:"message"`
	Objects string	`json:"objects"`
	Token string	`json:"token"`
	Chunks []chunk	`json:"chunks"`
}
// TODO change the naming server url

func Request(var dir string) {

	// ask for the file we want to download
	resp, err := http.Get("http://192.168.1.4:7000/upload?address=" + dir)

	defer resp.Body.Close()

	// get the json reply from the naming server
	body, err := ioutil.ReadAll(resp.Body)


}