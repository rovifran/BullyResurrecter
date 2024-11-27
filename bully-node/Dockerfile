FROM golang:alpine AS builder
LABEL intermediateStageToBeDeleted=true

# Change the working directory to /app instead of /build
WORKDIR /app

# Copy go.mod and go.sum files
COPY ./go.mod ./


# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Set GOCACHE
ENV GOCACHE=/root/.cache/go-build

# Build the application
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o bin/bully-node ./bully-node

FROM alpine:latest
COPY --from=builder /app/bin/bully-node /bully-node
ENTRYPOINT ["/bully-node"]