# ------------- actual usage -------------
APP_NAME := test

dev: 
	go run ./

prod: 
	go build -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME).exe ./

.PHONY: dev prod
