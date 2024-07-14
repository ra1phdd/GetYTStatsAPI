PHONY: test cover build

# Переменные
BUILD_DIR := build

# Юнит тесты и покрытие кода
test:
	go test -race -count 1 ./...

cover:
	go test -short -race -count 1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	rm coverage.out

# Сборка
build:
	mkdir -p $(BUILD_DIR)
	rm -rf $(BUILD_DIR)/*
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/GetYTStatsAPI_amd64 ./cmd/main/main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BUILD_DIR)/GetYTStatsAPI_arm64 ./cmd/main/main.go