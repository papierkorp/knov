# Variables
APP_NAME  := knov
BUILD     := $(shell date -u '+%Y')-$(shell git rev-list --count HEAD)-$(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M')
LAST_COMMIT_MSG := $(shell git log -1 --pretty=%s)
LDFLAGS   := -ldflags "-X 'knov/internal/version.Build=$(BUILD)' -X 'knov/internal/version.BuildTime=$(BUILD_TIME) UTC' -X 'knov/internal/version.LastCommitMessage=$(LAST_COMMIT_MSG)'"

# ------------- actual usage -------------
# make dev ARGS="--start-tests --remove"
dev: killdev swaggo-api-init changelog docs-templatedata env-example
	KNOV_LOG_LEVEL=debug go run ./ $(ARGS)

# same as dev, but runs inside the dev docker image - no local go/swag/gotext install needed
devd: docker-build-dev
	$(MAKE) docker-run-dev VOLUME=$(CURDIR):/app

prod: swaggo-api-init translation changelog docs-templatedata env-example
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME).exe ./


# ------------- docker -------------

docker-build-dev:
	docker build --no-cache -f tools/docker_dev/Dockerfile -t knov-dev .

docker-build-deployment:
	docker build --no-cache -f tools/docker_deployment/Dockerfile -t knov .

docker-run-dev:
	docker run --rm -it --name knov-dev -p 1324:1324 -v $(VOLUME) knov-dev

# ------------- helper -------------
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

tree:
	tree -I 'bin|data|data2|data3|storage|backups|knov_temp_test'

changelog:
	go run ./tools/genchangelog
	@git add docs/changelogs/ docs/releases/

# bump internal/version/version.yaml first, then: make release  (writes the release notes, commits, tags)
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

tempai:
	@echo "Creating tempai folder for AI context (flat structure)..."
	@rm -rf tempai
	@mkdir -p tempai
	@echo "Copying internal files (flattening subfolders)..."
	@find internal -type f \( -name "*.go" -o -name "*.tmpl" -o -name "*.json" -o -name "*.yaml" -o -name "*.yml" \) -exec cp {} tempai/ \;
	@echo "Copying root files..."
	@cp go.mod go.sum main.go Makefile styling.md tempai/ 2>/dev/null || true
	@cp ".env.example" "tempai/" 2>/dev/null || true
	@echo "Copying theme files with theme name prefix..."
	@for theme_dir in themes/*/; do \
		if [ -d "$$theme_dir" ]; then \
			theme_name=$$(basename "$$theme_dir"); \
			echo "  Processing theme: $$theme_name"; \
			find "$$theme_dir" -type f | while read file; do \
				filename=$$(basename "$$file"); \
				new_name="$${theme_name}-$${filename}"; \
				cp "$$file" "tempai/$$new_name"; \
			done; \
		fi \
	done
	@echo "Copying static/css files with 'static_' prefix..."
	@find static/css -type f | while read file; do \
		filename=$$(basename "$$file"); \
		new_name="static_$${filename}"; \
		cp "$$file" "tempai/$$new_name"; \
	done
	@echo "Renaming .gohtml to .html..."
	@for f in tempai/*.gohtml; do [ -f "$$f" ] && mv "$$f" "$${f%.gohtml}.html"; done
	@echo "Cleaning up"
	@rm -f tempai/*.exe tempai/*.log tempai/*.test
	@rm -f tempai/test-*
	@rm -f tempai/docs.go tempai/swagger.json tempai/swagger.yaml
	@rm -f tempai/*.gotext* tempai/catalog.go
	@echo "Creating file listing using tree command..."
	@make tree > tempai/FILE_LIST.txt 2>/dev/null || tree -I 'bin|data|data2|data3|storage|backups' > tempai/FILE_LIST.txt
	@echo ""
	@echo "tempai folder created successfully at ./tempai/"
	@echo "Total files: $$(ls -1 tempai/ | grep -v FILE_LIST.txt | wc -l)"
	@echo "Total size: $$(du -sh tempai | cut -f1)"
	@echo ""
	@echo "File naming conventions:"
	@echo "  Theme files:    {theme_name}-{original_filename}"
	@echo "  Static CSS:     static_{filename}"
	@echo "  Other files:    {original_filename}"
	@echo ""
	@echo "See tempai/FILE_LIST.txt for project structure (from 'make tree')"

# windows prod
# APP_NAME=knov && BUILD=$(date -u '+%Y')-$(git rev-list --count HEAD)-$(git rev-parse --short HEAD) && BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M') && LAST_COMMIT_MSG=$(git log -1 --pretty=%s) && swag init -g main.go -d . --exclude tempai -o internal/server/swagger && GOOS=windows GOARCH=amd64 go build -ldflags "-X 'knov/internal/version.Build=$BUILD' -X 'knov/internal/version.BuildTime=$BUILD_TIME UTC' -X 'knov/internal/version.LastCommitMessage=$LAST_COMMIT_MSG'" -o bin/knov.exe ./

# windows dev
#KNOV_LOG_LEVEL=debug go run ./

.PHONY: dev devd swaggo-api-init translation prod docker-build-dev docker-build-deployment docker-run-dev tree changelog release docs-templatedata env-example tempai killdev
