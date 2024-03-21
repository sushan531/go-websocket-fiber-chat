package main

import (
	"flag"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"log"
)

type miniClient map[string]map[string]*websocket.Conn // Modified type

type ClientObject struct {
	GROUP string
	USER  string
	conn  *websocket.Conn
}

type BroadcastObject struct {
	MSG  string
	FROM ClientObject
}

var clients = make(miniClient) // Initialized as a nested map
var register = make(chan ClientObject)
var broadcast = make(chan BroadcastObject)
var unregister = make(chan ClientObject)

func removeClient(org string, user string) {
	if conn, ok := clients[org][user]; ok { // Check if client exists
		delete(clients[org], user)
		conn.Close() // Close the connection before potentially removing the organization map
		if len(clients[org]) == 0 {
			delete(clients, org) // Remove empty organization map
		}
	}
}

func socketHandler() {
	for {
		select {
		case client := <-register:
			// Pre-initialize organization map if it doesn't exist
			if clients[client.GROUP] == nil {
				clients[client.GROUP] = make(map[string]*websocket.Conn)
			}
			clients[client.GROUP][client.USER] = client.conn
			log.Println("client registered:", client.GROUP, client.USER)

		case message := <-broadcast:
			for org, users := range clients {
				if org == message.FROM.GROUP {
					for user, conn := range users {
						if org != message.FROM.GROUP || user != message.FROM.USER {
							if err := conn.WriteMessage(websocket.TextMessage, []byte(message.MSG)); err != nil {
								log.Println("write error:", err)
								removeClient(org, user) // Update client removal
								conn.WriteMessage(websocket.CloseMessage, []byte{})
								conn.Close()
							}
						}
					}
				}
			}

		case client := <-unregister:
			removeClient(client.GROUP, client.USER) // Update client removal
			log.Println("client unregistered:", client.GROUP, client.USER)
		}
	}
}

func main() {
	app := fiber.New()

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			// Your authentication process goes here. Get the Token from header and validate it
			// Extract the claims from the token and set them to the Locals
			// This is because you cannot access headers in the websocket.Conn object below
			c.Locals("GROUP", string(c.Request().Header.Peek("GROUP")))
			c.Locals("USER", string(c.Request().Header.Peek("USER")))
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	go socketHandler()

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		clientObj := ClientObject{
			GROUP: c.Locals("GROUP").(string),
			USER:  c.Locals("USER").(string),
			conn:  c,
		}
		defer func() {
			unregister <- clientObj
			c.Close()
		}()

		// Register the client
		register <- clientObj

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return // Calls the deferred function, i.e. closes the connection on error
			}

			if messageType == websocket.TextMessage {
				// Broadcast the received message
				broadcast <- BroadcastObject{
					MSG:  string(message),
					FROM: clientObj,
				}
			} else {
				log.Println("websocket message received of type", messageType)
			}
		}
	}))

	addr := flag.String("addr", ":8080", "http service address")
	flag.Parse()
	app.Listen(*addr)
}
