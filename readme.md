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

## 👀 Author Notes

This project is designed to:

* demonstrate backend engineering skills in AI systems
* serve as a reference architecture for RAG systems
* evolve incrementally with production-grade features


