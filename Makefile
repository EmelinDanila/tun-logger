BINARY=tunlogger.exe

build:
	go mod tidy
	go build -o $(BINARY)

run: build
	./$(BINARY)

clean: 
	rm -f $(BINARY)
