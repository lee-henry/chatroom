package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

func main() {
	// 目標 WebSocket URL
	url := "ws://localhost:8080/ws"

	dialer := websocket.Dialer{}
	header := http.Header{} // 需要 header（例如 token）就加在這裡

	conn, resp, err := dialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			log.Printf("dial error, status=%d", resp.StatusCode)
		}
		log.Fatalf("dial error: %v", err)
	}
	defer conn.Close()
	log.Println("connected to server")

	// 寫一個訊息給 server
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello from client")); err != nil {
		log.Fatalf("write error: %v", err)
	}

	// 讀 server 回覆
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("read error: %v", err)
	}
	log.Printf("recv (type=%d): %s", msgType, msg)
}
