# Variables
APP_NAME := knov
THEME_DIR := themes

# ------------- actual usage -------------
dev: swaggo-api-init
	KNOV_LOG_LEVEL=debug go run ./

prod: clean swaggo-api-init translation
	go build -o bin/$(APP_NAME) ./
	GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME).exe ./

# ------------- theme packaging -------------

# Package all themes in themes/ directory (excluding builtin)
package-themes:
	@echo "Packaging themes..."
	@for theme in $(THEME_DIR)/*/ ; do \
		theme_name=$$(basename $$theme); \
		if [ "$$theme_name" != "builtin" ]; then \
			echo "Packaging $$theme_name..."; \
			cd $$theme && tar -czf ../$$theme_name.tgz theme.json templates/ static/ 2>/dev/null || tar -czf ../$$theme_name.tgz theme.json templates/ 2>/dev/null; \
			cd ..; \
			echo "Created $$theme_name.tgz"; \
		fi \
	done
	@echo "Theme packaging complete!"

# Package a specific theme
# Usage: make package-theme THEME=simple
package-theme:
	@if [ -z "$(THEME)" ]; then \
		echo "Error: THEME variable not set. Usage: make package-theme THEME=simple"; \
		exit 1; \
	fi
	@if [ ! -d "$(THEME_DIR)/$(THEME)" ]; then \
		echo "Error: Theme directory $(THEME_DIR)/$(THEME) does not exist"; \
		exit 1; \
	fi
	@echo "Packaging theme: $(THEME)..."
	@cd $(THEME_DIR)/$(THEME) && tar -czf ../$(THEME).tgz theme.json templates/ static/ 2>/dev/null || tar -czf ../$(THEME).tgz theme.json templates/
	@echo "Created $(THEME_DIR)/$(THEME).tgz"

# Clean packaged theme archives
clean-themes:
	@echo "Cleaning theme packages..."
	@rm -f $(THEME_DIR)/*.tgz
	@echo "Theme packages cleaned!"

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

.PHONY: dev swaggo-api-init translation prod docker docker-build docker-run clean package-themes package-theme clean-themes
