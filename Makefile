# Variables
APP_NAME := knov

# ------------- actual usage -------------
dev: swaggo-api-init templ-generate
	KNOV_LOG_LEVEL=debug go run ./cmd

prod: swaggo-api-init templ-generate translation
	go build -o bin/$(APP_NAME) ./cmd

# ------------- docker -------------

docker: docker-build docker-run

docker-build:
	docker build --no-cache -t knov-dev .

docker-run:
	docker run --rm -it --name knov-dev -p 1324:1324 -v /home/markus/develop/gitlab/gollum/tempwiki2:/data knov-dev

# ------------- helper -------------
rmt:
	rm ./themes/*.so

translation:
	cd internal/translation && go generate

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go -d . -o internal/server/api

.PHONY: templ-generate dev swaggo-api-init rmt translation prod docker docker-build docker-run
