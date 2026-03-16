``` mermaid
sequenceDiagram
    participant Client1
    participant Chat-service1
    participant Chat-service2
    participant Client2

    Client1->>Chat-service1: websocket [發送訊息]
    Chat-service1->>Chat-service2: kafka [轉發訊息]
    Chat-service1->>Client1: websocket [ack]
    Chat-service2->>Client2: websocket [發送訊息]
   
```