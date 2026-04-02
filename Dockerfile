## Stage 1: 构建前端
FROM node:22-alpine AS frontend-builder

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm run build

## Stage 2: 构建后端
FROM golang:1.25-alpine AS backend-builder

RUN apk --no-cache add ca-certificates tzdata
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

WORKDIR /app
COPY go.mod go.sum ./
ENV GOWORK=off
RUN go mod download
COPY . .

# 复制前端构建产物，供 go:embed 嵌入
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

RUN sqlc generate
RUN CGO_ENABLED=0 GOOS=linux go build -o /dash .

## Stage 3: 运行时
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /dash .
COPY config.yaml .

ARG GIT_COMMIT=unknown
RUN echo "${GIT_COMMIT}" > .commit

EXPOSE 8080
ENTRYPOINT ["/app/dash"]
