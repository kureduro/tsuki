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
		fmt.Println("please provide parameters in format host:port")
		return
	}

	connect := args[1]
	c, err := net.Dial("tcp", connect)
	defer c.Close()
	if err != nil {
		fmt.Println("Error while trying to connect to server with NAME:", err)
		return
	} else {
		fmt.Println("Connected successfuly")
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(">> ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Print("Error while reading the message with NAME:", err)
			return
		}

		// add the message to the buffer
		fmt.Fprintf(c, text+"\n")

		message, err := bufio.NewReader(c).ReadString('\n')

		if err != nil && strings.TrimSpace(string(text)) != "STOP" {
			fmt.Println("Error sending the message with NAME:", err)
			return
		}	
		if strings.TrimSpace(string(text)) == "STOP" {
			fmt.Println("TCP client is stopping.....")
			return	
		}
		fmt.Print("->: " + message)
	}
}
