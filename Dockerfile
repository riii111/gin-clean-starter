FROM golang:1.24.6-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM builder AS dev

RUN go install github.com/air-verse/air@latest && \
    go install gotest.tools/gotestsum@latest && \
    go install github.com/swaggo/swag/cmd/swag@latest

RUN curl -sSf https://atlasgo.sh | sh

WORKDIR /app
COPY . .

ENV GIN_MODE=debug
ENV PORT=8888

EXPOSE 8888

CMD ["air"]
