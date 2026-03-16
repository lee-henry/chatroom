``` mermaid
flowchart TD
    Client --> LoadBalancer
    LoadBalancer--> chat-server
    
    subgraph chat-server
        direction TB
        Auth-service
       User-service
       chat-service 
    end

    User-service --> User-DB(pgsql)
    chat-server-- 紀錄user與server訊息 -->Redis
    chat-service --> message-queue(kafka)
    message-queue --> message-DB(cassandra)
```













