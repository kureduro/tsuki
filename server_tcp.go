package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {

	args := os.Args

	if len(args) == 1 {
		fmt.Println("Please provide the port number")
		return
	}
	port := ":" + args[1]
	l, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	c, err := l.Accept()

	defer c.Close()

	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}

		if strings.TrimSpace(string(netData)) == "STOP" {
			fmt.Println("exiting...")
			return
		}
		fmt.Print("->", string(netData))
		t := time.Now()
		myTime := t.Format(time.RFC3339) + "\n"
		c.Write([]byte(myTime))
	}
}
