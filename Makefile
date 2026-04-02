.PHONY: build build-linux run dev test lint deploy hash migrate templ css css-watch

TAILWIND_BIN := ./bin/tailwindcss

$(TAILWIND_BIN):
	@mkdir -p bin
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
	chmod +x tailwindcss-linux-x64
	mv tailwindcss-linux-x64 $(TAILWIND_BIN)

templ:
	templ generate

css: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i static/input.css -o static/styles.css --minify

css-watch: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i static/input.css -o static/styles.css --watch

build: templ css
	go build -o voicetask .

build-linux: templ css
	GOOS=linux GOARCH=amd64 go build -o voicetask .

run:
	go run .

dev:
	templ generate --watch &
	$(MAKE) css-watch &
	air

test:
	go test ./...

lint:
	golangci-lint run ./...

deploy: build-linux
	scp voicetask root@$(SERVER):/opt/voicetask/voicetask
	ssh root@$(SERVER) "systemctl restart voicetask"

hash:
	@go run ./cmd/hashpass "$(PASS)"

migrate:
	ssh root@$(SERVER) "sudo -u postgres psql voicetask < /opt/voicetask/migrations/001_create_tasks.sql"
