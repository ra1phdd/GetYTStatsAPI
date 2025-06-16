.PHONY: test cover build protos

# Переменные
BUILD_DIR := build

# Юнит тесты и покрытие кода
test:
	go test -race -count 1 ./...

cover:
	go test -short -race -count 1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	rm coverage.out

protos:
	protoc --go_out=. --go-grpc_out=. protos/server_v1.proto
	protoc --go_out=. --go-grpc_out=. protos/client_v1.proto

# Сборка
build:
	mkdir -p $(BUILD_DIR)
	rm -rf $(BUILD_DIR)/*
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_linux_amd64 ./cmd/main/main.go
	GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_linux_i386 ./cmd/main/main.go
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_linux_arm ./cmd/main/main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_linux_arm64 ./cmd/main/main.go
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_windows_amd64.exe ./cmd/main/main.go
	GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -o $(BUILD_DIR)/main_windows_i386.exe ./cmd/main/main.go