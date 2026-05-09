# RAG Platform (Go + pgvector)

A modular, production-oriented Retrieval-Augmented Generation (RAG) system built in Go.

**Production-oriented Agentic RAG backend implemented in Go**

A modular, extensible Retrieval-Augmented Generation (RAG) system designed with real-world constraints in mind: hybrid retrieval, query rewriting, conversation memory, model routing, and fault tolerance.

---

## 🚀 Overview

`rag-infra-go` is not a demo project. It is a **backend system for building scalable AI applications** that combines:

* Retrieval (vector + structured)
* LLM orchestration
* Stateful conversations
* Ingestion pipelines
* Observability & fault tolerance

The goal is to demonstrate how to build a **production-ready RAG system**, not just call an LLM.

---

## 🧩 Core Features

### 🔍 Hybrid Retrieval

* Vector search (semantic)
* Optional keyword / structured retrieval
* Deduplication + reranking pipeline

### 🧠 Query Processing

* Query rewriting (context-aware)
* Multi-turn conversation support
* Question decomposition (future extension)

### 💬 Conversation Memory

* Sliding window memory
* Optional summarization for long contexts
* Token-aware context management

### ⚙️ Model Routing & Fallback

* Multiple LLM providers (OpenAI, Ollama, etc.)
* Priority-based routing
* Timeout + fallback handling
* Circuit breaker (planned)

### 📦 Document Ingestion Pipeline

* File parsing
* Text cleaning
* Chunking
* Embedding
* Vector storage

### 📡 Streaming Responses

* Server-Sent Events (SSE)
* Token streaming from LLM

### 📊 Observability (Planned)

* Trace IDs per request
* Step-by-step pipeline logging

---

## 🏗️ Architecture

### High-Level Flow

```
User Query
→ Load Memory
→ Query Rewriting
→ Retrieval (Hybrid)
→ Reranking
→ Context Assembly
→ LLM Generation (Streaming)
→ Response
```

---

## 📁 Project Structure

```
rag-infra-go/
├── cmd/
│   └── api/                # Application entrypoint
│
├── internal/
│   ├── chat/               # Chat orchestration (main pipeline)
│   ├── retrieval/          # Retrievers (vector, keyword)
│   ├── ingestion/          # Document pipeline
│   ├── memory/             # Conversation memory
│   ├── providers/          # LLM + embedding providers
│   ├── routing/            # Model routing & fallback
│   ├── rerank/             # Reranking logic
│   ├── rewrite/            # Query rewriting
│   ├── trace/              # Request tracing
│   └── transport/          # HTTP / SSE handlers
│
├── pkg/
│   ├── llm/                # LLM client interfaces
│   ├── vectorstore/        # Vector DB abstraction
│   ├── embedding/          # Embedding interface
│   └── pipeline/           # Pipeline primitives
│
├── configs/                # Configuration files
├── scripts/                # Dev / setup scripts
└── README.md
```

---

## 🧠 Core Design Concepts

### 1. Pipeline-Oriented Architecture

Each request is processed as a sequence of independent stages:

* Rewriting
* Retrieval
* Reranking
* Generation

Each stage is:

* Replaceable
* Testable
* Observable

---

### 2. Interface-Driven Design

Key interfaces:

```go
type Retriever interface {
    Retrieve(ctx context.Context, query string) ([]Document, error)
}

type Reranker interface {
    Rerank(ctx context.Context, query string, docs []Document) ([]Document, error)
}

type LLMClient interface {
    Generate(ctx context.Context, prompt string) (string, error)
    Stream(ctx context.Context, prompt string) (<-chan Token, error)
}

type EmbeddingClient interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

type MemoryStore interface {
    Load(ctx context.Context, sessionID string) ([]Message, error)
    Save(ctx context.Context, sessionID string, msg Message) error
}
```

---

### 3. Hybrid Retrieval Strategy

Instead of relying on a single retriever:

* Vector retrieval → high recall
* Structured/intent retrieval → high precision

Then:

* Merge results
* Deduplicate
* Rerank (optional cross-encoder)

---

### 4. Model Routing Strategy

Supports multiple providers:

```
Primary Model → Timeout → Fallback Model → Local Model
```

Features:

* First-response timeout
* Priority-based routing
* Future: health checks + circuit breaker

---

### 5. Ingestion Pipeline

Document processing is implemented as a pipeline:

```
Upload → Parse → Clean → Chunk → Embed → Store
```

Each step is modular and replaceable.

---

## ⚙️ Tech Stack

* **Language**: Go
* **HTTP**: net/http (or Gin/Fiber optional)
* **Vector DB**: Milvus / pgvector (pluggable)
* **LLM Providers**: OpenAI / Ollama / others
* **Streaming**: SSE
* **Storage**: PostgreSQL / Redis (optional)

---

## 🧪 Example API

### Chat Endpoint

```
POST /api/chat
```

Request:

```json
{
  "session_id": "123",
  "query": "What is RAG?"
}
```

Response (streaming):

```
data: token1
data: token2
...
```

---

## 🛠️ Getting Started

### 1. Clone the repo

```
git clone https://github.com/yourname/rag-infra-go.git
cd rag-infra-go
```

### 2. Setup environment

```
cp .env.example .env
```

Fill in:

```
OPENAI_API_KEY=...
```

### 3. Run the server

```
go run ./cmd/api
```

---

## 📈 Roadmap

* [ ] Reranking with cross-encoder
* [ ] Circuit breaker for model routing
* [ ] Background ingestion workers
* [ ] MCP / tool calling support
* [ ] Admin APIs for knowledge base
* [ ] Distributed tracing (OpenTelemetry)
* [ ] Docker + local stack setup

---

## 🎯 Why This Project

Most RAG examples focus on:

* embeddings
* vector search
* calling an LLM

This project focuses on:

* **system design**
* **reliability**
* **extensibility**
* **real-world constraints**

---

## 🤝 Inspiration

Inspired by modern enterprise RAG systems and production AI architectures.

---

## ⭐ If you find this useful

Give it a star ⭐ — it helps others discover the project.

---
## 🚀 Production Deployment (Docker + Subdirectory Hosting)

This section describes how to deploy the system in production using Docker Compose, including:

- PostgreSQL + pgvector (persistent storage)
- Go backend API
- React frontend served via Nginx under a subpath (e.g. `/rag`)

---

### 🧱 Architecture


- Frontend and backend are served under the same domain and subpath
- Nginx proxies `/rag/api/*` → backend `/api/*`
- No CORS issues in production (same-origin)
```
browser
  ↓
frontend nginx container at /rag
  ↓ 
/rag/api/*
backend container
  ↓
pgvector db container

```
### 📁 Project Structure
For actual server deployment, I’d use one top-level compose that includes:

```
db
backend
frontend/nginx

```
Best structure:
```
deploy/
  docker-compose.yml
  .env

rag-infra-go/
  Dockerfile

interview-copilot-rag/
    Dockerfile
    nginx.conf.template

```
So frontend is not “separate” operationally — it is just built from the frontend repo and included in the top-level compose.


### ⚙️ Environment Configuration

Create `deploy/.env`:

```env
# ============================================================
# App
# ============================================================
APP_ENV=PROD
APP_BASE_PATH=/rag

FRONTEND_PORT=8000
BACKEND_PORT=8080

# ============================================================
# Database
# ============================================================
POSTGRES_DB=rag_platform
POSTGRES_USER=rag_user
POSTGRES_PASSWORD=change_me
POSTGRES_VOLUME_DIR=/srv/rag/postgres

# ============================================================
# Backend DB config
# ============================================================
DB_HOST=db
DB_PORT=5432
DB_USER=rag_user
DB_PASSWORD=change_me
DB_NAME=rag_platform
DB_SSLMODE=disable

# ============================================================
# Auth
# ============================================================
ADMIN_USERNAME=admin
ADMIN_PASSWORD=change_me
ADMIN_EMAIL=you@example.com

SESSION_COOKIE_NAME=interview_copilot_session
SESSION_TTL_HOURS=24
SECURE_COOKIES=true

# ============================================================
# Google OAuth
# ============================================================
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

GOOGLE_REDIRECT_URL=https://your-domain.com/rag/api/auth/google/callback
FRONTEND_POST_LOGIN_URL=https://your-domain.com/rag/

# ============================================================
# CORS
# ============================================================
CORS_ALLOWED_ORIGINS=https://your-domain.com
CORS_ALLOW_CREDENTIALS=true

# ============================================================
# LLM
# ============================================================
LLM_PROVIDER=openai
OPENAI_API_KEY=
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_CHAT_MODEL=gpt-4o-mini

# ============================================================
# Embeddings
# ============================================================
EMBEDDING_PROVIDER=local
LOCAL_EMBEDDING_URL=
LOCAL_EMBEDDING_API_KEY=
LOCAL_EMBEDDING_MODEL=nomic-embed-text-v2-moe:latest
LOCAL_EMBEDDING_DIM=0
ENABLE_HNSW_INDEX=true

# ============================================================
# Reranker
# ============================================================
RERANKER_PROVIDER=none
RERANKER_TOP_K=5

# ============================================================
# Frontend build
# ============================================================
VITE_APP_BASE_PATH=/rag
VITE_API_BASE_PATH=/rag
````

---

### 🐳 Docker Compose

Create `deploy/docker-compose.yml`:

```yaml
services:
  db:
    image: pgvector/pgvector:0.8.2-pg18-trixie
    container_name: rag-db
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - ${POSTGRES_VOLUME_DIR}:/var/lib/postgresql
    ports:
      - "5432:5432"

  backend:
    build:
      context: ../rag-infra-go
      dockerfile: Dockerfile
    container_name: rag-backend
    restart: unless-stopped
    depends_on:
      - db
    environment:
      APP_ENV: ${APP_ENV}
      PORT: ${BACKEND_PORT}

      DB_HOST: db
      DB_PORT: 5432
      DB_USER: ${DB_USER}
      DB_PASSWORD: ${DB_PASSWORD}
      DB_NAME: ${DB_NAME}
      DB_SSLMODE: ${DB_SSLMODE}

      ADMIN_USERNAME: ${ADMIN_USERNAME}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      ADMIN_EMAIL: ${ADMIN_EMAIL}

      SESSION_COOKIE_NAME: ${SESSION_COOKIE_NAME}
      SESSION_TTL_HOURS: ${SESSION_TTL_HOURS}
      SECURE_COOKIES: ${SECURE_COOKIES}

      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID}
      GOOGLE_CLIENT_SECRET: ${GOOGLE_CLIENT_SECRET}
      GOOGLE_REDIRECT_URL: ${GOOGLE_REDIRECT_URL}
      FRONTEND_POST_LOGIN_URL: ${FRONTEND_POST_LOGIN_URL}

      CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS}
      CORS_ALLOW_CREDENTIALS: ${CORS_ALLOW_CREDENTIALS}

      OPENAI_API_KEY: ${OPENAI_API_KEY}
      OPENAI_BASE_URL: ${OPENAI_BASE_URL}
      OPENAI_CHAT_MODEL: ${OPENAI_CHAT_MODEL}

      EMBEDDING_PROVIDER: ${EMBEDDING_PROVIDER}
      LOCAL_EMBEDDING_URL: ${LOCAL_EMBEDDING_URL}
      LOCAL_EMBEDDING_API_KEY: ${LOCAL_EMBEDDING_API_KEY}
      LOCAL_EMBEDDING_MODEL: ${LOCAL_EMBEDDING_MODEL}
      LOCAL_EMBEDDING_DIM: ${LOCAL_EMBEDDING_DIM}
      ENABLE_HNSW_INDEX: ${ENABLE_HNSW_INDEX}

      RERANKER_PROVIDER: ${RERANKER_PROVIDER}
      RERANKER_TOP_K: ${RERANKER_TOP_K}
    ports:
      - "${BACKEND_PORT}:8080"
    volumes:
      - /srv/rag/uploads:/app/uploads

  frontend:
    build:
      context: ../interview-copilot-rag
      dockerfile: Dockerfile
      args:
        VITE_APP_BASE_PATH: ${VITE_APP_BASE_PATH}
        VITE_API_BASE_PATH: ${VITE_API_BASE_PATH}
    container_name: rag-frontend
    restart: unless-stopped
    depends_on:
      - backend
    environment:
      APP_BASE_PATH: ${APP_BASE_PATH}
      BACKEND_UPSTREAM: http://backend:8080
    ports:
      - "${FRONTEND_PORT}:80"
```

---

### 🌐 Nginx Configuration (Frontend)

`nginx.conf.template`:

```nginx
server {
    listen 80;

    location ${APP_BASE_PATH}/api/ {
        proxy_pass ${BACKEND_UPSTREAM}/api/;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_buffering off;
    }

    location ${APP_BASE_PATH}/ {
        root /usr/share/nginx/html;
        try_files $uri $uri/ ${APP_BASE_PATH}/index.html;
    }

    location = ${APP_BASE_PATH} {
        return 301 ${APP_BASE_PATH}/;
    }
}
```

---

### ▶️ Run Deployment

```bash
cd deploy
docker compose up -d --build
```

---

### 🌍 Access

```text
http://your-server:8000/rag
```

API:

```text
http://your-server:8000/rag/api/healthz
```

---

### 📝 Notes

* Change `/rag` to any path via `APP_BASE_PATH`
* Database persists via `POSTGRES_VOLUME_DIR`
* Frontend and backend share same origin → no CORS issues
* Google OAuth redirect must match the subpath (`/rag`)




## 👀 Author Notes

This project is designed to:

* demonstrate backend engineering skills in AI systems
* serve as a reference architecture for RAG systems
* evolve incrementally with production-grade features


