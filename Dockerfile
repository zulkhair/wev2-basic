################################
# Build Image - Normal
################################
FROM golang:1.24 AS build

ARG SOURCE_PATH

WORKDIR /go/src/app

# Set Go environment variables for private repositories
ENV GOPRIVATE=github.com/argus-labs/*,pkg.world.dev/*
ENV GOSUMDB=off
ENV GOPROXY=direct

# Copy go.mod and go.sum files first to leverage Docker layer caching
COPY /${SOURCE_PATH}/go.mod /${SOURCE_PATH}/go.sum ./

# Download dependencies
RUN go mod download

# Set the GOCACHE environment variable to /root/.cache/go-build to speed up build
ENV GOCACHE=/root/.cache/go-build

# Copy the entire source code
COPY /${SOURCE_PATH} ./

# Build the binary
RUN --mount=type=cache,target="/root/.cache/go-build" go build -v -o /go/bin/app

################################
# Runtime Image - Normal
################################
FROM gcr.io/distroless/base-debian12 AS runtime

# Copy the binary from the build image
COPY --from=build /go/bin/app /usr/bin

# Run the binary
CMD ["app"]