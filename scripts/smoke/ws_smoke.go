package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	url := "ws://127.0.0.1:8080/api/v1/ws/live-buses?busIds=smoke-bus-1"
	headers := http.Header{}
	headers.Set("X-Request-ID", "ws-go-smoke")

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.Dial(url, headers)
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("dial failed: http=%s body=%s\n", resp.Status, string(body))
		}
		log.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("connected")

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("read: %v\n", err)
		return
	}
	fmt.Printf("recv: %s\n", string(msg))
}
