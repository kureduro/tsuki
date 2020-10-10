package main

import (
    "fmt"
    "bufio"
    "os"
    "strings"
)
func help() {
    // TODO print a list of instructions (maybe from a file would be more elegant)
    fmt.Println("exit : leave the program") 
}

func download(parameters []string) {
    fmt.Println(parameters)
}
func upload(parameters []string) {
    fmt.Println(parameters)
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
        default:
            fmt.Println("unknown command please try again or write help for help")
        }
        //clear the paramteres slice
        parameters = nil
    }
}
