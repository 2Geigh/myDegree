BINARY_NAME=scraper.out
BINARY_PATH=../../bin/${BINARY_NAME}

ALL: clean build

build:
	go build -o ${BINARY_PATH} main.go

run:
	go run main.go

clean:
	go clean
	go mod tidy
	rm -f ${BINARY_PATH}