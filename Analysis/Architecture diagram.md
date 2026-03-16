``` mermaid
flowchart TD
    Client --> LoadBalancer
    LoadBalancer --> chat-service
    LoadBalancer --> Auth-service
    LoadBalancer --> User-service
    User-service --> User-DB(pgsql)
    chat-service --> message-queue(kafka)
    message-queue --> message-DB(cassandra)
    Redis
```