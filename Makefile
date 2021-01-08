NAME ?= controller
VERSION ?= latest

# default list of platforms for which multiarch image is built
ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/amd64,linux/arm64"
endif

## if IMG_RESULT is unspecified, by default the image will be pushed to registry
#ifeq (${IMG_RESULT}, load)
#	export PUSH_ARG="--load"
#    # if load is specified, image will be built only for the build machine architecture.
#    export PLATFORMS="local"
#else ifeq (${IMG_RESULT}, cache)
#	# if cache is specified, image will only be available in the build cache, it won't be pushed or loaded
#	# therefore no PUSH_ARG will be specified
#else
#	export PUSH_ARG="--push"
#endif

run:
	go run ./main.go
test: fmt
	go test ./...

manager: fmt
	go build -o bin/manager main.go

fmt:
	go fmt ./...

docker-build:
	docker build . -t ${NAME}:${VERSION} && kind load docker-image ${NAME}:${VERSION} --name mw

start-pod:
	kubectl apply -f config/controller/controller.yml

refresh-pod:
	kubectl delete pod manager && kubectl apply -f config/controller/controller.yml

