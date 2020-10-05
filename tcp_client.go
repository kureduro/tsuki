package main

// Ask about the stop/STOP case :3
import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {

	args := os.Args
	if len(args) == 1 {
		fmt.Println("please provide host:port")
		return
	}

	connect := args[1]
	c, err := net.Dial("tcp", connect)

	if err != nil {
		fmt.Println(err)
		return

	}

	for {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print(">> ")
		text, _ := reader.ReadString('\n')
		fmt.Fprintf(c, text+"\n")

		message, _ := bufio.NewReader(c).ReadString('\n')
		fmt.Print("->: " + message)
		if strings.TrimSpace(string(text)) == "STOP" {
			fmt.Println("TCP client is stopping.....")
			return

		}
	}
}
