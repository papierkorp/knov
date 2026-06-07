# Variables
APP_NAME := knov

# ------------- actual usage -------------
dev: swaggo-api-init
	KNOV_LOG_LEVEL=debug go run ./

prod: swaggo-api-init translation
	go build -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME).exe ./

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
	swag init -g main.go -d . --exclude tempai -o internal/server/swagger

tree:
	tree -I 'bin|data|data2|data3|storage'

tempai:
	@echo "Creating tempai folder for AI context (flat structure)..."
	@rm -rf tempai
	@mkdir -p tempai
	@echo "Copying internal files (flattening subfolders)..."
	@find internal -type f \( -name "*.go" -o -name "*.tmpl" -o -name "*.json" -o -name "*.yaml" -o -name "*.yml" \) -exec cp {} tempai/ \;
	@echo "Copying root files..."
	@cp Dockerfile go.mod go.sum main.go Makefile styling.md tempai/ 2>/dev/null || true
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
	@echo "Copying static/generate-translations.sh with 'static_' prefix..."
	@cp static/generate-translations.sh tempai/static_generate-translations.sh 2>/dev/null || true
	@echo "Renaming .gohtml to .html..."
	@for f in tempai/*.gohtml; do [ -f "$$f" ] && mv "$$f" "$${f%.gohtml}.html"; done
	@echo "Cleaning up"
	@rm -f tempai/*.exe tempai/*.log tempai/*.test
	@rm -f tempai/test-*
	@rm -f tempai/docs.go tempai/swagger.json tempai/swagger.yaml
	@rm -f tempai/*.gotext* tempai/catalog.go
	@echo "Creating file listing using tree command..."
	@make tree > tempai/FILE_LIST.txt 2>/dev/null || tree -I 'bin|data|data2|data3|storage' > tempai/FILE_LIST.txt
	@echo ""
	@echo "tempai folder created successfully at ./tempai/"
	@echo "Total files: $$(ls -1 tempai/ | grep -v FILE_LIST.txt | wc -l)"
	@echo "Total size: $$(du -sh tempai | cut -f1)"
	@echo ""
	@echo "File naming conventions:"
	@echo "  Theme files:    {theme_name}-{original_filename}"
	@echo "  Static CSS:     static_{filename}"
	@echo "  Static script:  static_generate-translations.sh"
	@echo "  Other files:    {original_filename}"
	@echo ""
	@echo "See tempai/FILE_LIST.txt for project structure (from 'make tree')"

# windows prod
# swag init -g main.go -d . --exclude tempai -o internal/server/swagger
# cd internal/translation && go generate && cd ../..
# go build -o bin/knov ./
# GOOS=windows GOARCH=amd64 go build -o bin/knov.exe ./

# windows dev
# swag init -g main.go -d . --exclude tempai -o internal/server/swagger
# KNOV_LOG_LEVEL=debug go run ./

.PHONY: dev dev-fast swaggo-api-init translation prod docker docker-build docker-run tree tempai
