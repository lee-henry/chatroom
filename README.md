# chatroom
# 海量用戶即時通訊系統設計筆記

> 模擬面試練習紀錄

---

## 一、需求釐清

### 功能性需求（Functional Requirements）

| 功能 | 說明 |
|------|------|
| 通訊方式 | 1對1私訊 |
| 歷史訊息 | 保留至少1年 |
| 已讀回條 | 需要 |
| 查詢聊天紀錄 | 依時間排序 |
| 訊息類型 | 文字 + 圖片（不含影片） |

### 非功能性需求（Non-Functional Requirements）

| 指標 | 目標值 |
|------|-------|
| 訊息延遲 | < 500ms |
| 系統可用性 | 99.99% uptime |
| 訊息持久性 | 絕對不能丟失 |
| 服務範圍 | 台灣地區 |

### 記憶技巧：SLADG

```
S - Scalability   → 規模多大？
L - Latency       → 速度要多快？
A - Availability  → 掛掉能接受嗎？
D - Durability    → 資料能丟嗎？
G - Geo           → 哪些地區？
```

---

## 二、容量估算（Capacity Estimation）

```
DAU：5,000 萬
每人每天：40 則訊息

每日訊息量：5000萬 × 40 = 20億則

QPS：20億 ÷ 86400 ≈ 23,000 QPS
Peak QPS：23,000 × 3 ≈ 70,000 QPS

每則訊息大小：
  內容：45 bytes（中文15字 × 3 bytes）
  Metadata：50 bytes
  合計：~100 bytes

每日儲存：20億 × 100 bytes = 200 GB
一年儲存：200 GB × 365 ≈ 73 TB
```

---

## 三、整體系統架構

```
用戶
  │（WebSocket）
Load Balancer
  │
Chat Server（多台）
  │              │
Kafka          Redis
                │（查詢用戶在哪台Server）
              Chat Server（目標）
  │
Cassandra（訊息儲存）
```

---

## 四、關鍵技術決策

### 4.1 通訊協議：WebSocket

| 協議 | 適合場景 | 缺點 |
|------|---------|------|
| HTTP Polling | 簡單實作 | 浪費資源、延遲高 |
| Long Polling | 較即時 | Server 資源消耗大 |
| **WebSocket** | **雙向即時通訊** | **連線管理複雜** |
| SSE | Server → Client 單向 | 無法雙向 |

**斷線策略（心跳機制）：**
```
每 30 秒 Client 送一個 ping
Server 沒收到 → 判定離線 → 斷開連線

Client 偵測到斷線 → 自動重連
websocket.onclose = () => {
  setTimeout(() => reconnect(), 3000)
}
```

### 4.2 Redis 的用途

```
1. Session Store：記錄用戶連在哪台 Server
   Key: user:{user_id}:server → "chat-server-03"

2. 在線狀態：
   Key: user:{user_id}:online → TTL 30秒（心跳更新）

3. 最近訊息快取：加速讀取
```

### 4.3 訊息跨 Server 傳遞

```
情境：Alice 連在 Server A，Bob 連在 Server B

流程：
Alice 發訊息到 Server A
  → Server A 查 Redis 得知 Bob 在 Server B
  → Server A 寫入 Kafka
  → Server B 從 Kafka 消費訊息
  → Server B 推送給 Bob
```

```
Alice          Server A         Kafka          Server B        Bob
  │                │               │               │            │
  │──傳訊息────────▶│               │               │            │
  │                │──寫入Kafka────▶│               │            │
  │◀───ACK─────────│               │──通知Server B─▶│            │
  │                │               │               │──推送訊息──▶│
```

---

## 五、離線處理

### Bob 離線的完整流程

```
Alice 發訊息
  → Server A 查 Redis：Bob 離線
  → 訊息存入 Cassandra（持久化）
  → 發送 Push Notification（APNs / FCM）

Bob 上線後：
  → 連上 WebSocket
  → 告訴 Server 最後一則訊息的 ID
  → Server 從 DB 撈出之後所有未讀訊息
  → 推送給 Bob
```

### 線上 vs 離線差異

| 情境 | 處理方式 |
|------|---------|
| Bob 在線 | Server 直接透過 WebSocket 推送 |
| Bob 離線 | 存 DB + 發 Push Notification |
| Bob 重新上線 | 主動 Pull 未讀訊息 |

> ⚠️ Push Notification 不能直接帶訊息內容（隱私 + 大小限制）
> 只帶通知：「你有 3 則未讀訊息」，開 App 後再拉取內容

---

## 六、Cassandra Schema 設計

### 為什麼選 Cassandra？

- 寫入量極大（70,000 QPS）→ Cassandra 寫入效能極佳
- 訊息天然按 `(conversation_id, timestamp)` 查詢
- 水平擴展容易

### Schema

```sql
CREATE TABLE messages (
  conversation_id  UUID,
  bucket           INT,       -- 時間分桶，例如：202403
  message_id       ULID,      -- 帶時間序的唯一ID
  sender_id        BIGINT,
  content          TEXT,
  status           TINYINT,   -- 0:sent, 1:delivered, 2:read
  PRIMARY KEY ((conversation_id, bucket), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC);
```

### Primary Key 說明

```
PRIMARY KEY ((conversation_id, bucket), message_id)
              └──── Partition Key ────┘  └─ Clustering Key ─┘
              決定存在哪台機器              決定 partition 內排序
```

### 為什麼需要 Bucket？

```
問題：Hot Partition
  所有訊息的 Partition Key 都是 conv_123
  → 全部流量打到同一台機器
  → 其他機器閒置，失去分散式的意義

解法：加入時間分桶
  (conv_123, 202401) → Node A
  (conv_123, 202402) → Node B
  (conv_123, 202403) → Node C
  → 流量平均分散到各節點
```

### 跨月查詢

```
Cassandra 只能保證同一 partition 內的排序
跨 partition 需要應用層自行合併排序：

1. 查 bucket = 202403 → 拿到20則
2. 不夠50則，繼續查 bucket = 202402
3. 在程式裡合併排序後回傳
```

---

## 七、高可用設計（99.99% Uptime）

### 99.99% 是什麼概念？

```
一年總時間 = 525,600 分鐘

99%     → 可停機 5,256 分鐘 ≈ 87 小時
99.9%   → 可停機 525 分鐘  ≈ 8.7 小時
99.99%  → 可停機 52 分鐘
99.999% → 可停機 5 分鐘
```

### 可用性計算

**串聯系統（任一掛掉就整體掛掉）：**
```
整體可用性 = 各元件可用性相乘
99.99% × 99.99% × 99.99% × 99.99% = 99.96%
→ 元件越多，整體可用性越低！
```

**並聯系統（全部掛掉才會失敗）：**
```
兩台同時掛掉機率 = 0.01% × 0.01% = 0.000001%
→ 多台 Server 大幅提升可用性！
```

### Chat Server 掛掉的處理流程

```
Load Balancer 偵測心跳失敗
  ├── 停止導流到掛掉的 Server
  ├── 用戶 WebSocket 斷線 → 自動重連到其他 Server
  ├── Kafka 保證訊息不丟失
  ├── Auto Scaling 自動起新的 Server
  └── 發警報通知開發者
```

### 消除單點故障（SPOF）

```
❌ 危險：只有一台就是單點故障
✅ 正確：每個元件都要備援

Load Balancer  → 主備兩台
Chat Server    → 最少3台 + Auto Scaling
Kafka          → 多個 Broker + 副本
Cassandra      → 多節點 + Replication Factor 3
機房            → 至少兩個可用區（AZ）
```

### 監控指標

| 指標 | 警戒值 |
|------|-------|
| Server CPU | > 80% |
| 訊息延遲 | > 500ms |
| WebSocket 連線數 | 接近上限 |
| Kafka 積壓量 | 持續增加 |
| 錯誤率 | > 0.1% |

---

## 八、Load Balancer 概念

### 是什麼？

> Load Balancer 是「服務台」，把進來的請求平均分配給多台 Server

### 三大功能

```
1. 分流      → 把請求平均分給多台 Server
2. 健康檢查  → 每隔幾秒確認每台 Server 是否存活
3. 自動剔除  → 掛掉的 Server 自動移出，新 Server 自動加入
```

### 分流策略

| 策略 | 說明 | 適合場景 |
|------|------|---------|
| Round Robin | 輪流分配 | 一般 HTTP |
| Least Connection | 導向連線數最少的 Server | **WebSocket** ✅ |
| IP Hash | 同 IP 永遠到同一台 | 需要 Session 的情境 |

> WebSocket 連線會持續很久，使用 Least Connection 確保每台 Server 負載平均

### 常見工具

| 類型 | 例子 | 適合誰 |
|------|------|-------|
| 雲端服務 | AWS ALB、GCP Load Balancer | 不想自己維運 |
| 軟體自架 | Nginx、HAProxy | 想要自己控制 |
| 硬體 | F5 | 大型企業 |

### Nginx 基本設定概念

```nginx
upstream chat_servers {
    least_conn;              # 最少連線策略
    server chat-server-1:8080;
    server chat-server-2:8080;
    server chat-server-3:8080;
}

server {
    listen 80;
    location / {
        proxy_pass http://chat_servers;
    }
}
```

---

## 九、實作學習路線

```
Phase 1 - 基礎版（2-3週）
├── WebSocket Server（Node.js 或 Go）
├── 簡單的訊息路由
└── Redis Session Store

Phase 2 - 進階版（3-4週）
├── 引入 Kafka 做訊息佇列
├── Cassandra 訊息儲存
└── 訊息送達確認機制（ACK）

Phase 3 - 高可用版（4-6週）
├── 多節點 Chat Server + Load Balancer
├── Auto Scaling 設計
├── 監控與 Alerting
└── Push Notification（APNs / FCM）
```

---
