# Stage 1: Build
FROM golang:1.25.6-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY ./webhook/main.go .

# Build completamente estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -tags netgo \
    -a \
    -installsuffix cgo \
    -o webhook-service .

# Stage 2: Runtime
FROM alpine:latest AS production

ARG APP_UID
ARG APP_GID

# Instalar apenas o necessário
RUN apk --no-cache add ca-certificates wget

# Criar grupo e usuário (sintaxe Alpine)
RUN addgroup -g ${APP_GID} gouser && \
    adduser -D -u ${APP_UID} -G gouser gouser

# Copiar binário com permissões
COPY --from=builder --chown=${APP_UID}:${APP_GID} /build/webhook-service /app/webhook-service

# Tornar executável
RUN chmod 755 /app/webhook-service

# Criar diretório webhook_jobs
RUN mkdir -p /app/webhook_jobs && \
    chown -R ${APP_UID}:${APP_GID} /app/webhook_jobs && \
    chmod 775 /app/webhook_jobs

WORKDIR /app

USER gouser

EXPOSE 8000

CMD ["/app/webhook-service"]