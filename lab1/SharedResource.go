package main

import (
	"fmt"
	"net"
	"os"
)

/* A simple function to verify error */
func CheckError(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(0)
	}
}

func main() {
	ServerAddr, err := net.ResolveUDPAddr("udp", ":10001")
	CheckError(err)

	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	CheckError(err)
	defer ServerConn.Close()

	buf := make([]byte, 4096)
	for {
		n, _, err := ServerConn.ReadFromUDP(buf)
		if string(buf[0:n]) == "x" {
			fmt.Println("A process is trying to access the Critical Section")
		}
		fmt.Println("Received ", string(buf[0:n]))
		if err != nil {
			fmt.Println("Error: ", err)
		}
	}
}
