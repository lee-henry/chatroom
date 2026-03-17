package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// upgrader 用於將一般的 HTTP 連線升級成 WebSocket 連線
var upgrader = websocket.Upgrader{
	// ReadBufferSize 與 WriteBufferSize 分別設定了 WebSocket 讀出與寫入的緩衝區大小 (Byte)
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin 用於驗證發出 WebSocket 連線請求的來源 (Origin)。
	// 安全性考量：若回傳 false，連線將被拒絕 (回傳 HTTP 403)。
	CheckOrigin: func(r *http.Request) bool {
		// 開發時先全部放行 (回傳 true)，正式環境應該要做嚴格的 Origin 驗證，避免 CSRF 或阻擋未授權網域
		return true
	},
}

// handleWebSocket 是一個 HTTP Handler，負責處理傳入的 WebSocket 連線請求
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 嘗試將當下的 HTTP 請求升級為 WebSocket 雙向連線
	// 回傳的 conn 是與該客戶端成功建立的 WebSocket 連線物件
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("升級 WebSocket 失敗: %v", err)
		return
	}
	// 確保函式結束時 (無論是正常結束或發生錯誤退出)，都會關閉這條 WebSocket 連線，以防資源洩漏
	defer conn.Close()

	log.Printf("客戶端已連線: %s", conn.RemoteAddr())

	// 進入無窮迴圈，持續監聽、接收客戶端丟過來的訊息
	for {
		// ReadMessage 會卡住 (阻塞) 在這裡，直到收到新訊息、發生錯誤或連線被關閉
		// msgType: 表示這段訊息的類型 (例如 1=文字 TextMessage, 2=二進位 BinaryMessage)
		// msg: 實際收到的訊息內容，資料型態為 byte 切片
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			// 如果發生讀取錯誤（最常見的就是客戶端主動斷線、網路異常等），就跳出迴圈
			log.Printf("讀取錯誤 (可能已斷線): %v", err)
			break
		}
		log.Printf("收到訊息: %s", msg)

		// 回傳 Echo 給 client (將收到的訊息原封不動再回傳給客戶端)
		// WriteMessage 負責將指定類型與內容的資料寫回給連線的客戶端
		if err := conn.WriteMessage(msgType, msg); err != nil {
			log.Printf("寫入錯誤: %v", err)
			break
		}
	}
}

func main() {
	// 註冊 HTTP 路由：當有人存取 "/ws" 時，就呼叫 handleWebSocket 這個函式來處理
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("WebSocket server 啟動於 :8080/ws")
	// 啟動 HTTP 伺服器並監聽在本地的 8080 通訊埠
	// ListenAndServe 會持續阻擋住 main 函式的結束，直到伺服器崩潰或被關閉為止
	if err := http.ListenAndServe(":8080", nil); err != nil {
		// 如果無法啟動 (例如 port 被佔用)，印出錯誤並中斷程式
		log.Fatal(err)
	}
}
