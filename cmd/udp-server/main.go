package main

import "project/internal/udp"

func main() {
	if err := udp.RunServer(":9001"); err != nil {
		panic(err)
	}
}
