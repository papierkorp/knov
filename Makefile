# Variables
APP_NAME := knov
BUILD_TAGS := fts5

# ------------- actual usage -------------
dev: swaggo-api-init
	KNOV_LOG_LEVEL=debug go run -tags "$(BUILD_TAGS)" ./

dev-fast: swaggo-api-init
	KNOV_LOG_LEVEL=debug go run ./

prod: clean swaggo-api-init translation
	go build -tags "$(BUILD_TAGS)" -o bin/$(APP_NAME) ./
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -tags "$(BUILD_TAGS)" -o bin/$(APP_NAME).exe ./

# ------------- docker -------------

docker: docker-build docker-run

docker-build:
	docker build --no-cache -t knov-dev .

docker-run:
	docker run --rm -it --name knov-dev -p 1324:1324 -v /home/markus/develop/gitlab/gollum/tempwiki2:/data knov-dev

# ------------- helper -------------
translation:
	cd internal/translation && go generate

swaggo-api-init:
	swag init -g main.go -d . -o internal/server/swagger

tree:
	tree -I 'bin|data|storage'

.PHONY: dev dev-fast swaggo-api-init translation prod docker docker-build docker-run tree
