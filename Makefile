.PHONY: build build-linux run dev test lint deploy hash migrate

build:
	go build -o voicetask .

build-linux:
	GOOS=linux GOARCH=amd64 go build -o voicetask .

run:
	go run .

dev:
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
