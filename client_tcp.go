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
	defer c.Close()
	if err != nil {
		fmt.Println(err)
		return
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Print(err)
			return
		}
		fmt.Fprintf(c, text+"\n")

		message, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Print("->: " + message)
		if strings.TrimSpace(string(text)) == "stop" {
			fmt.Println("TCP client is stopping.....")
			return

		}
	}
}
