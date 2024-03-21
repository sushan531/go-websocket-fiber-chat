package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var addr = "ws://localhost:8080/ws" // Replace with the server's address

func main() {
	orgPtr := flag.String("group", "", "Your organization name")
	userPtr := flag.String("user", "", "Your user name")
	flag.Parse()

	if *orgPtr == "" || *userPtr == "" {
		log.Fatal("Please provide both organization and user name using -group and -user flags")
	}

	header := make(http.Header)
	header.Add("GROUP", *orgPtr)
	header.Add("USER", *userPtr)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		// Add other configuration options as needed
	}

	conn, _, err := dialer.Dial(addr, header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			fmt.Printf("\nReceived: %s\n", message)
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		// Non-blocking message retrieval from user
		fmt.Print("Enter message to send (or 'quit' to exit): ")
		message, err := reader.ReadString('\n')
		message = strings.TrimSpace(message)

		if err != nil {
			log.Println("Error reading input:", err)
			break
		}

		if message == "quit" {
			break
		}

		err = conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			log.Println("write:", err)
			break
		}
	}

	<-done
}
