package main

import (
	"fmt"
	"encoding/json"
)

type Chunk struct {
	StorageIP string `json:"storageIP"`
	ChunkID string 	`json:"chunkID"`
}
type downloadRequestAns struct {
	Status string	`json:"status"`
	Message string	`json:"message"`
	Token string	`json:"token"`
	Chunks []Chunk	`json:"chunks"`
}

func main() {
	s := string(`{"status":"OK","message":"Go upload there","token":"44d09f62-690f-2d2f-6a1e-65167465cc4b","chunks":[{"chunkID":"11380cd2-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380d4a-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380d68-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380da4-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380e26-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"}]}`)
	data := downloadRequestAns{}

	json.Unmarshal([]byte(s), &data)
	
	fmt.Printf("status is: %s", data.Status)
}