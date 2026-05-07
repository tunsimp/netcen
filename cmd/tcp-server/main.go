package main

import "project/internal/tcp"

func main() {
	if err := tcp.RunServer(":9000"); err != nil {
		panic(err)
	}
}
