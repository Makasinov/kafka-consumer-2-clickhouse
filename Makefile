#!make

PROJECT_PATH=`pwd`
PROJECT_NAME=kafka-consumer
MAIN_FILE_PATH=cmd/kafka-consumer

## run: Launch app with -race flag
run:
	@echo " > Launch app..."
	@bash -c "go run -race cmd/kafka-consumer/main.go config/config.json"

## install: install dependencies from go.mod & go.sum file
install:
	@echo " > Downloading dependencies..."
	@bash -c "go mod download"
	@echo " > Done"

## test: Launch unit tests
test:
	@echo " > Launch unit tests..."
	@bash -c "go test ./... -v -bench=."
	@echo " > Done"

## reinstall: Fully reinstall dependencies from go.mod & go.sum file
reinstall:
	@rm go.sum
	@bash -c "make install"

## prepare: Download ClickHouse dependencies
prepare:
	# https://repo.yandex.ru/clickhouse/deb/stable/main
	@wget https://repo.yandex.ru/clickhouse/deb/stable/main/clickhouse-common-static_20.3.5.21_amd64.deb -O resources/clickhouse-common-static.deb
	@wget https://repo.yandex.ru/clickhouse/deb/stable/main/clickhouse-server-base_1.1.54370_amd64.deb -O resources/clickhouse-server-base.deb
	@wget https://repo.yandex.ru/clickhouse/deb/stable/main/clickhouse-common-dbg_1.1.54370_amd64.deb -O resources/clickhouse-common-dbg.deb
	@wget https://repo.yandex.ru/clickhouse/deb/stable/main/clickhouse-client_20.3.5.21_all.deb -O resources/clickhouse-client.deb

## container-build: Building container with name kafka-consumer
container-build:
	@docker build -t kafka-consumer:latest .

## container-run: Running container with name kafka-consumer
container-run:
	(docker stop kafka-consumer || true)
	(docker rm kafka-consumer || true)
	@docker run --name kafka-consumer -p 8080:8080 -v `pwd`/config/:/var/log/kafka-consumer/ kafka-consumer /etc/kafka-consumer/conf.d/config.json

help: Makefile
	@echo " > Help commands list "$(PROJECT_NAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
