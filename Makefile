.PHONY:
.SILENT:

build:
	go build -o ./.bin/bot cmd/main.go

run: build
	./.bin/bot

# make run