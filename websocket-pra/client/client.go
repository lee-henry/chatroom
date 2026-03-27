package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

func main() {
	// 目標 WebSocket 伺服器的 URL 位址
	// 使用 ws:// 前綴代表非加密的 WebSocket 連線 (若有 SSL 則為 wss://)
	url := "ws://localhost:8080/ws"

	// 初始化 Dialer，它是用來幫我們建立 WebSocket 連線的客戶端撥號器
	dialer := websocket.Dialer{}

	// 設定 HTTP 請求標頭 (Header)
	// 若伺服器需要驗證身份(例如帶 JWT Token 或 Cookie)，可以使用 header.Add("Key", "Value") 加入
	header := http.Header{}

	chat(url, header, dialer)
}

func chat(url string, header http.Header, dialer websocket.Dialer) {
	// 發起 WebSocket 握手 (Handshake) 與連線
	// conn: 成功建立的 WebSocket 連線物件
	// resp: 伺服器回傳的 HTTP 握手回應 (包含狀態碼、Header 等)
	// err: 發生錯誤時的錯誤資訊
	conn, resp, err := dialer.Dial(url, header)
	if err != nil {
		// 如果發生錯誤，且伺服器有回傳 HTTP 回應 (例如 401 Unauthorized 或 404 Not Found)
		if resp != nil {
			log.Printf("連線失敗，HTTP 狀態碼=%d", resp.StatusCode)
		}
		log.Fatalf("Dial 連線錯誤: %v", err) // 印出錯誤並立刻結束程式
	}

	// 使用 defer 確保程式或函式結束時，一定會關閉這條 WebSocket 連線，以釋放資源
	defer conn.Close()
	log.Println("已成功連線至 WebSocket 伺服器")
	log.Println("可以開始輸入訊息")
	log.Println("輸入 exit 即可退出")
	log.Println("--------------------------------")

	reader := bufio.NewReader(os.Stdin)
	for {

		// ----- 傳送訊息給伺服器 -----
		// websocket.TextMessage 代表我們要送純文字，第二個參數是將字串轉為 byte 陣列的實際資料

		// 從終端讀取數據
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("輸入失敗")
		}

		// 如果輸入exit 就退出
		line = strings.Trim(line, "\n")
		line = strings.Trim(line, "\r")
		if line == "exit" {
			fmt.Println("客戶端退出")
			break
		}

		if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			log.Fatalf("寫入/傳送訊息錯誤: %v", err)
		}

		// ----- 讀取伺服器回傳的訊息 -----
		// ReadMessage() 會持續等待 (阻塞) 直到收到伺服器的下一則訊息為止
		// msgType: 訊息格式 (例如 TextMessage 或 BinaryMessage)
		// msg: 收到的實際資料內容 (byte 陣列)
		_, _, err = conn.ReadMessage()
		if err != nil {
			log.Fatalf("讀取訊息錯誤: %v", err)
		}
		// 將收到的 byte 資料轉作字串印出
		// log.Printf("收到回覆 (訊息類型=%d): %s", msgType, msg)
	}

}
