# Variables
APP_NAME := knov

# ------------- actual usage -------------
dev: swaggo-api-init 
	KNOV_LOG_LEVEL=info go run ./

prod: clean swaggo-api-init translation
	go build -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME).exe ./

# ------------- docker -------------

docker: docker-build docker-run

docker-build:
	docker build --no-cache -t knov-dev .

docker-run:
	docker run --rm -it --name knov-dev -p 1324:1324 -v /home/markus/develop/gitlab/gollum/tempwiki2:/data knov-dev

# ------------- helper -------------
clean:
	rm -f ./themes/*.so && rm -rf ./bin

translation:
	cd internal/translation && go generate

swaggo-api-init:
	swag init -g main.go -d . -o internal/server/swagger

.PHONY: dev swaggo-api-init rmt translation prod docker docker-build docker-run clean
