# Kafka-Consumer

## Envs

| Variable | Description                         |
|:---------|:------------------------------------|
| Debug    | (default: false) Enables debug mode |

## Endpoints

| Path | Description   |
|:-----|:---|
| /metrics     |  For prometheus  |

Run only in container (can use `$ make container-build && make container-run`)  
Mount volume to use config and see all consumed batches `-v $(pwd)/config/:/var/log/kafka-consumer/`
