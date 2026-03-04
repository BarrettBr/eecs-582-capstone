# Use alpine for a slimmer image
FROM golang:1.25-alpine
WORKDIR /app

# Install some nice to have tools in case you ssh into the container
RUN apk update && apk upgrade && \
    apk add --no-cache bash git 

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY ./ingest/go.mod ./ingest/go.sum ./
RUN go mod download

# Copy in the dependencies
COPY ./ingest .

# Build the go app
RUN go build -o main .

# Run the app
CMD ["./main"]
