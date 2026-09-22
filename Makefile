# Variables
APP_NAME        := knov
BUILD           := $(shell date -u '+%Y')-$(shell git rev-list --count HEAD)-$(shell git rev-parse --short HEAD)
BUILD_TIME      := $(shell date -u '+%Y-%m-%d %H:%M')
LAST_COMMIT_MSG := $(shell git log -1 --pretty=%s)
LDFLAGS         := -ldflags "-X 'knov/internal/version.Build=$(BUILD)' -X 'knov/internal/version.BuildTime=$(BUILD_TIME) UTC' -X 'knov/internal/version.LastCommitMessage=$(LAST_COMMIT_MSG)'"
SWAG_VERSION   := v1.16.6
GOTEXT_VERSION := v0.41.0
ANDROID_JAVA_HOME ?= $(JAVA_HOME)
GRADLE_VERSION := $(shell sed -n 's#.*gradle-\([0-9.]*\)-bin\.zip#\1#p' android/gradle/wrapper/gradle-wrapper.properties)

# ------------- actual usage -------------
# make dev ARGS="--start-tests --remove"
dev: killdev swaggo-api-init changelog docs-templatedata env-example
	KNOV_LOG_LEVEL=debug go run ./ $(ARGS)

# same as dev, but inside the dev docker image - no local go/swag/gotext install needed
devd: docker-build-dev
	$(MAKE) docker-run-dev VOLUME=$(CURDIR):/app

# linux/amd64 + windows/amd64 + linux/arm64 (Termux on android) builds
prod: swaggo-api-init translation changelog docs-templatedata env-example
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME).exe ./
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(APP_NAME)-arm64 ./

# debug apk for the android wrapper app: go server as a jniLibs native library, then gradle.
# arm64-v8a only - sideload-only personal tool, no store pressure to cover every ABI
mobile-apk: swaggo-api-init translation
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build $(LDFLAGS) -o android/app/src/main/jniLibs/arm64-v8a/libknovserver.so ./
	@test -n "$(ANDROID_JAVA_HOME)" || { echo "no JDK found - set ANDROID_JAVA_HOME (see docs/developer.md)"; exit 1; }
	@ls $${GRADLE_USER_HOME:-$$HOME/.gradle}/wrapper/dists/gradle-$(GRADLE_VERSION)-bin/*/gradle-$(GRADLE_VERSION) >/dev/null 2>&1 || test -n "$(ALLOW_GRADLE_DOWNLOAD)" || { \
		echo "gradle $(GRADLE_VERSION) isn't installed locally - this would download it (~150MB) now."; \
		echo "use 'make docker-build-apk' instead, or rerun with ALLOW_GRADLE_DOWNLOAD=1 to download it here anyway."; \
		exit 1; \
	}
	# clean, not assembleDebug: gradle can leave a previous .so's bytes as dead weight in the apk zip
	cd android && JAVA_HOME=$(ANDROID_JAVA_HOME) ./gradlew clean assembleDebug
	@echo "apk: android/app/build/outputs/apk/debug/app-debug.apk"

docker-build-dev:
	docker build --no-cache -f tools/docker_dev/Dockerfile -t knov-dev .

docker-build-deployment:
	docker build --no-cache -f tools/docker_deployment/Dockerfile -t knov .

# gradle cache is a named volume so it survives across --rm runs
docker-build-apk: docker-prepare-android-image
	docker run --rm -v $(CURDIR):/app -v knov-android-gradle:/root/.gradle knov-android

# ------------- helper -------------

install-tools:
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	go install golang.org/x/text/cmd/gotext@$(GOTEXT_VERSION)

translation:
	cd internal/translation && go generate

swaggo-api-init:
	swag init -g main.go -d . --exclude tempai -o internal/server/swagger

docs-templatedata:
	go run ./tools/gentemplatedocs
	@git add docs/template_data.md

env-example:
	go run ./tools/genenv
	@git add .env.example

changelog:
	go run ./tools/genchangelog
	@git add docs/changelogs/ docs/releases/

tree:
	tree -I 'bin|data|data2|data3|storage|backups|knov_temp_test'

# bump internal/version/version.yaml first, then: make release
release:
	@set -e; \
	VERSION=$$(sed -n 's/^version:[[:space:]]*//p' internal/version/version.yaml | tr -d '"'); \
	test -n "$$VERSION" || { echo "no version in internal/version/version.yaml"; exit 1; }; \
	TAG="v$$VERSION"; \
	git rev-parse -q --verify "refs/tags/$$TAG" >/dev/null && { echo "tag $$TAG already exists"; exit 1; } || true; \
	go run ./tools/genchangelog -version "$$TAG"; \
	git add internal/version/version.yaml docs/changelogs/ docs/releases/; \
	git commit -m "chore: release $$TAG"; \
	git tag -a "$$TAG" -m "release $$TAG"; \
	echo "created release commit + tag $$TAG - push with: git push --follow-tags"

killdev:
	-fuser -k 1324/tcp

docker-run-dev:
	docker run --rm -it --name knov-dev -p 1324:1324 -v $(VOLUME) knov-dev

# builds the image android apks get built inside, for machines without a local android sdk
docker-prepare-android-image:
	docker image inspect knov-dev >/dev/null 2>&1 || $(MAKE) docker-build-dev
	docker build -f tools/docker_android/Dockerfile -t knov-android .

test:
	rm -rf ./temp-knov
	mkdir ./temp-knov
	$(MAKE) prod && cp bin/$(APP_NAME)* temp-knov/ && cp .env.example temp-knov/.env
	@echo "cd temp-knov"

# windows prod
# APP_NAME=knov && BUILD=$(date -u '+%Y')-$(git rev-list --count HEAD)-$(git rev-parse --short HEAD) && BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M') && LAST_COMMIT_MSG=$(git log -1 --pretty=%s) && swag init -g main.go -d . --exclude tempai -o internal/server/swagger && GOOS=windows GOARCH=amd64 go build -ldflags "-X 'knov/internal/version.Build=$BUILD' -X 'knov/internal/version.BuildTime=$BUILD_TIME UTC' -X 'knov/internal/version.LastCommitMessage=$LAST_COMMIT_MSG'" -o bin/knov.exe ./

# windows dev
#KNOV_LOG_LEVEL=debug go run ./

.PHONY: dev devd swaggo-api-init install-tools translation prod mobile-apk docker-build-dev docker-build-deployment docker-run-dev docker-prepare-android-image docker-build-apk tree changelog release docs-templatedata env-example killdev test
