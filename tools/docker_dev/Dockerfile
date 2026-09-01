FROM golang:1.26.4

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    go install golang.org/x/text/cmd/gotext@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV KNOV_LOG_LEVEL=debug
ENV KNOV_DATA_PATH=/data

RUN git config --global --add safe.directory $KNOV_DATA_PATH && \
    git config --global --add safe.directory /app

EXPOSE 1324

CMD ["make", "dev"]
