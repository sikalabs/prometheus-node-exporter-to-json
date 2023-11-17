TAG = v0.1.0
IMAGE = sikalabs/prometheus-node-exporter-to-json
IMAGE_LATEST = ${IMAGE}:latest
IMAGE_TAGGED = ${IMAGE}:${TAG}

run:
	go run main.go

docker-all:
	@make docker-build
	@make docker-push

docker-build:
	docker build --platform linux/amd64 -t ${IMAGE_LATEST} -t ${IMAGE_TAGGED} .

docker-push:
	docker push ${IMAGE_LATEST}
	docker push ${IMAGE_TAGGED}
