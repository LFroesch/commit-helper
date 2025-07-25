build:
	go build -o git-helper main.go

cp:
	cp git-helper ~/.local/bin/
	
install: build cp