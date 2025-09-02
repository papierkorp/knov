# Variables
APP_NAME := knov

# ------------- actual usage -------------
dev: setup-test-data swaggo-api-init templ-generate
	KNOV_LOG_LEVEL=debug go run ./cmd

prod: swaggo-api-init templ-generate translation
	go build -o bin/$(APP_NAME) ./cmd

# ------------- test data setup -------------
setup-test-data: copy-test-files create-git-operations

copy-test-files:
	@echo "Setting up test data..."
	@mkdir -p data
	@cp -r data_testfiles/* data/
	@echo "Test files copied to data folder"

create-git-operations:
	@echo "Creating git operations for testing..."
	@cd data && \
	if [ ! -d .git ]; then git init; fi && \
	git add . && \
	git commit -m "Initial test data" --allow-empty || true && \
	echo "# Test File Created by Make" > test_created_file.md && \
	git add test_created_file.md && \
	git commit -m "Add dynamically created test file" || true && \
	mkdir -p projects && \
	(mv ai.md projects/ 2>/dev/null || echo "ai.md already moved or doesn't exist") && \
	git add . && \
	git commit -m "Move ai.md to projects folder" --allow-empty || true && \
	echo "# Another Test File" > projects/project_notes.md && \
	git add projects/project_notes.md && \
	git commit -m "Add project notes" || true && \
	rm -f test_created_file.md && \
	git add . && \
	git commit -m "Remove test file" --allow-empty || true && \
	echo "Git operations completed"

clean-test-data:
	@echo "Cleaning test data..."
	@rm -rf data/*
	@echo "Test data cleaned"

# ------------- helper -------------
rmt:
	rm ./themes/*.so

translation:
	cd internal/translation && go generate

templ-generate:
	TEMPL_EXPERIMENT=rawgo templ generate

swaggo-api-init:
	swag init -g cmd/main.go -d . -o internal/server/api

.PHONY: templ-generate dev swaggo-api-init rmt translation setup-test-data copy-test-files create-git-operations clean-test-data
