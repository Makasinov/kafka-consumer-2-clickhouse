#!make

PROJECT_PATH=`pwd`
PROJECT_NAME=kafka-consumer
MAIN_FILE_PATH=cmd/kafka-consumer

## run: Запускает приложение в дебаг режиме (с флагом -race)
run:
	@echo " > Идет запуск..."
	@bash -c "go run -race cmd/kafka-consumer/main.go config/config.json"

## install: Устанавливает зависимости из go.mod файла
install:
	@echo " > Идёт скачивание зависимостей"
	@bash -c "go mod download"
	@echo " > Выполено"

## test: Запуск тестов
test:
	@echo " > Запуск Unit тестов"
	@bash -c "go test ./... -v -bench=."
	@echo " > Выполнено"

## reinstall: Перекачиват и обновляет все зависимости до последней версии
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

container-build:
	@docker build -t kafka-consumer:latest .

container-run:
	(docker stop kafka-consumer || true)
	(docker rm kafka-consumer || true)
	@docker run --name kafka-consumer -p 8080:8080 -v `pwd`/config/:/var/log/kafka-consumer/ kafka-consumer /etc/kafka-consumer/conf.d/config.json

help: Makefile
	@echo " > Список команд по "$(PROJECT_NAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
