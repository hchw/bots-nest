FROM node:20-alpine AS frontend
WORKDIR /app
COPY web/ui/package*.json ./
RUN npm ci
COPY web/ui/ .
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/dist ./web/ui/dist
RUN CGO_ENABLED=0 go build -o bots-nest ./cmd/

FROM alpine:3.19
WORKDIR /app
COPY --from=backend /app/bots-nest .
COPY config.yaml .
EXPOSE 8080
CMD ["./bots-nest"]
