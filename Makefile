# Variables
APP_NAME := knov

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

dev: templ-generate
	go build -o ./bin/$(APP_NAME) ./cmd/main.go && air

build: templ-generate
	go build -o ./bin/$(APP_NAME) ./cmd/main.go

docker-dev-build:
	docker build --network=host --no-cache -t knovault-dev -f Dockerfile_dev .

# Use shell command to get absolute path
PWD := $(shell pwd)

docker-dev-run:
	docker run -it --rm \
	-v "${PWD}:/app" \
	-p 1323:1323 \
	-w /app \
	--add-host=proxy.golang.org:172.217.22.113 \
	knovault-dev

.PHONY: templ-generate dev build docker-dev-build docker-dev-run
