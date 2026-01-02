graph TB
    subgraph "Client Layer"
        WEB[Web Application]
        IOS[iOS App]
        AND[Android App]
        DESK[Desktop App]
    end

    subgraph "CDN & Load Balancing"
        CDN[CDN - Static Assets]
        LB[Load Balancer / Nginx]
    end

    subgraph "API Gateway Layer"
        GW[API Gateway<br/>Kong/AWS API Gateway]
        WSS[WebSocket Gateway]
    end

    subgraph "Authentication & Security"
        AUTH[Auth Service<br/>OAuth2, JWT, 2FA]
        KMS[Key Management Service<br/>E2EE Keys]
        RATE[Rate Limiter]
    end

    subgraph "Core Services"
        USER[User Service<br/>Profile, Settings]
        MSG[Messaging Service<br/>Text, Media]
        PRES[Presence Service<br/>Online Status]
        CALL[Call Service<br/>WebRTC Signaling]
        NOTIF[Notification Service<br/>Push, Email, SMS]
        SEARCH[Search Service<br/>Elasticsearch]
    end

    subgraph "AI Services"
        AIGATE[AI Gateway]
        CHATBOT[Chatbot Service]
        TRANS[Translation Service]
        STT[Speech-to-Text]
        SMART[Smart Reply]
        SENTI[Sentiment Analysis]
    end

    subgraph "Module Services"
        FILE[File Service<br/>Upload/Download]
        MEET[Meeting Service<br/>Scheduling]
        TASK[Task Management]
        PAY[Payment Service]
        ANAL[Analytics Service]
    end

    subgraph "Media Layer"
        MEDIA[Media Server<br/>Mediasoup/Jitsi]
        TURN[TURN/STUN Servers]
        REC[Recording Service]
    end

    subgraph "Data Layer"
        subgraph "Databases"
            USERDB[(User DB<br/>PostgreSQL)]
            MSGDB[(Message DB<br/>Cassandra)]
            FILEDB[(File Metadata<br/>MongoDB)]
            CACHEDB[(Redis Cache<br/>& Sessions)]
        end
        
        subgraph "Message Queue"
            KAFKA[Kafka/RabbitMQ]
        end
        
        subgraph "Storage"
            S3[Object Storage<br/>S3/MinIO]
            ES[Elasticsearch]
        end
    end

    subgraph "Monitoring & Logging"
        PROM[Prometheus]
        GRAF[Grafana]
        ELK[ELK Stack]
        TRACE[Jaeger Tracing]
    end

    WEB --> CDN
    IOS --> LB
    AND --> LB
    DESK --> LB
    CDN --> LB

    LB --> GW
    LB --> WSS

    GW --> RATE
    RATE --> AUTH

    AUTH --> USER
    AUTH --> MSG
    AUTH --> CALL

    GW --> USER
    GW --> MSG
    GW --> PRES
    GW --> SEARCH
    GW --> FILE
    GW --> MEET
    GW --> TASK
    GW --> PAY

    WSS --> MSG
    WSS --> PRES
    WSS --> CALL

    MSG --> KMS
    MSG --> NOTIF
    MSG --> KAFKA

    CALL --> MEDIA
    MEDIA --> TURN
    CALL --> REC

    GW --> AIGATE
    AIGATE --> CHATBOT
    AIGATE --> TRANS
    AIGATE --> STT
    AIGATE --> SMART
    AIGATE --> SENTI

    USER --> USERDB
    USER --> CACHEDB
    MSG --> MSGDB
    MSG --> CACHEDB
    SEARCH --> ES
    FILE --> FILEDB
    FILE --> S3

    NOTIF --> KAFKA
    ANAL --> KAFKA

    USER -.-> PROM
    MSG -.-> PROM
    CALL -.-> PROM
    PROM -.-> GRAF

    USER -.-> ELK
    MSG -.-> ELK
    CALL -.-> ELK