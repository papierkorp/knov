FROM golang:1.26.4

WORKDIR /app

COPY Makefile ./
RUN make install-tools

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV KNOV_LOG_LEVEL=debug
ENV KNOV_DATA_PATH=/data

RUN git config --global --add safe.directory $KNOV_DATA_PATH && \
    git config --global --add safe.directory /app

EXPOSE 1324

CMD ["make", "dev"]
