package main

import (
    "fmt"
    "bufio"
    "os"
	"io/ioutil"
    "strings"
    "strconv"
    "path"
    "net/http"
    "encoding/json"
)

var curDir = "/"

func help() {
    // TODO print a list of instructions (maybe from a file would be more elegant)
    fmt.Println("exit : leave the program") 
}


// Upload code 

type Chunk struct {
    StorageIP string `json:"storageIP"`
	ChunkID string 	`json:"chunkID"`
}
type RequestAns struct {
    Status string	`json:"status"`
	Message string	`json:"message"`
    Token string	`json:"token"`
    Objects []string `json:"objects"`
	Chunks []Chunk	`json:"chunks"`
}

func download(params []string) {
    if len(params) > 2 {
        fmt.Println("Usage : download full_file_directory or download file_name\n type help to see available commands")
        return
    }
}
//     // TODO make file_name a full dir
//     dir := params[1]
//     resp, err := http.Get("http://192.168.1.4:7000/download?&address=" + dir)
// 	if err != nil {
//         fmt.Println("Error connecting to the server please try again later...")
//         return
// 	}
//     // resp now has the json file
//     // TODO check if it needs conversion
// 	data := RequestAns{}
//     json.Unmarshal([]byte(resp), &data)
//     if data.Status != "OK" {
//         // TODO change the message if needed to a better one 
//         fmt.Println("Server rejected the request...")
//         return
//     }
//     // TODO write download code
// }
func upload(params []string) {
    if len(params) != 3 {
        fmt.Println("Usage : upload local_file_directory remote_directory\n type help to see available commands")
        return
    }
    // TODO handle the case with dir + name in params[2] already
    dir := params[1]
    // get the size of the file
    fi, err := os.Stat(dir)
    dir = path.Join(params[1],params[2])
    if err != nil {
        fmt.Println("Error reading file, make sure it exists and not broken...")
        return
    }
    // get the size
    // size in Kb make it  in bytes
    // ask naming server if you can upload
    // NOTE + TODO change the server address
    // TODO add ability to upload to a specific folder (other than the current one in command line)
    /// TODO *1024
    size := strconv.FormatInt(fi.Size(), 10)
    fmt.Println(size)
    resp, err := http.Get("http://10.91.55.196:7070/upload?size=" + size + "&address=" + dir)
	if err != nil {
        fmt.Println("Error connecting to the server please try again later...")
        return
	}
    // resp now has the json file
    // TODO check if it needs conversion
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    data := RequestAns{}
    json.Unmarshal([]byte(bodyBytes), &data)
    if data.Status != "OK" {
        // TODO change the message if needed to a better one 
        fmt.Println("Server rejected the file...")
        return
    }
    // reply example (TODO delete)
    //s := string(`{"status":"OK","message":"Go upload there","token":"44d09f62-690f-2d2f-6a1e-65167465cc4b","chunks":[{"chunkID":"11380cd2-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380d4a-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380d68-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380da4-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"},{"chunkID":"11380e26-0ae5-11eb-801f-367dda11f678","storageIP":"10.91.84.229"}]}`)

    // TODO write upload code
}
 // TODO use path for everything lol
 // TODO ask for name server address and port from the beginning
func createDir(params []string) {
    if len(params) > 2 {
        fmt.Println("usage: mkdir folder_name")
        return
    }
    
}
func openDir(params [] string) {
    if len(params) > 2 {
        fmt.Println("usage : cd dir or cd")
        return
    }
    var dir string
    if len(params) == 1 {
        dir = "/"
    } else {
        dir = curDir + params[1]
    }
    resp, err := http.Get("http://10.91.55.196:7070/cd?address=" + dir)
	if err != nil {
        fmt.Println("Error connecting to the server please try again later...")
        return
    }
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    data := RequestAns{}
    json.Unmarshal([]byte(bodyBytes), &data)
    if data.Status != "OK" {
        fmt.Println("Error: ", data.Message)
        fmt.Printf("The current dir is still %s\n", curDir)
        return
    }

    curDir = dir
    fmt.Printf("The current dir is %s\n", curDir)
}
func listDir(params [] string) {
    if len(params) > 2 {
        fmt.Println("usage: ls or ls dir")
        return
    }
    var dir string
    if len(params) == 1 {
        // TODO add current dir here
        dir = curDir
        // TODO add go back one dir case
    } else {
        dir = params[1]
    }
    resp, err := http.Get("http://10.91.55.196:7070/ls?address=" + dir)
	if err != nil {
        fmt.Println("Error connecting to the server please try again later...")
        return
    }
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    data := RequestAns{}
    json.Unmarshal([]byte(bodyBytes), &data)
    if data.Status != "OK" {
        fmt.Println("Error: ", data.Message)
        return
    }

    for _, name := range data.Objects {
        fmt.Println(name)
    }
}
func main() {
    fmt.Println("Welcome to client software write help for help")
    reader := bufio.NewReader(os.Stdin)
    var parameters []string

    for {
        fmt.Print(">>> ")
        text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Print("Error while reading user input with NAME:", err)
			return
        }

        parameters = append(strings.Fields(text))
        // TODO delete later
        fmt.Println("-->", parameters[0])
        switch parameters[0]{
        case "exit":
            fmt.Println("Exiting the program...")
            return
        case "help":
            help()
        case "download":
            download(parameters)
        case "upload":
            upload(parameters)
        case "mkdir":
            createDir(parameters)
        case "cd":
            openDir(parameters)
        case "ls":
            listDir(parameters)
        default:
            fmt.Println("unknown command please try again or write help for help")
        }
        //clear the paramteres slice
        parameters = nil
    }
}
