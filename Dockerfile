FROM golang:alpine AS builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o main .

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/main .

# Build a small image
FROM scratch

COPY --from=builder /dist/main /

# Command to run
ENTRYPOINT ["/main"]

#FROM golang:1.15 as builder
#
#ARG TARGETPLATFORM
#
#WORKDIR /workspace
#
#ENV GO111MODULE=on \
#    CGO_ENABLE=0
#
#COPY go.mod go.sum ./
#
#RUN go mod download
#
## Copy the go source
#COPY . .
#
## Build
#RUN export GOOS=$(echo ${TARGETPLATFORM} | cut -d / -f1) && \
#  export GOARCH=$(echo ${TARGETPLATFORM} | cut -d / -f2) && \
#  GOARM=$(echo ${TARGETPLATFORM} | cut -d / -f3 | cut -c2-) && \
#  go build -a -o manager main.go
#
## Use distroless as minimal base image to package the manager binary
## Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
#
#WORKDIR /
#
#COPY --from=builder /workspace/manager .
#
#USER nonroot:nonroot
#
#ENTRYPOINT ["/manager"]
