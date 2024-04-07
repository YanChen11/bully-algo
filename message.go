package main

type Message struct {
	From        string
	Type        int
}

const (
	Ping    = iota
	Pong
	Election
	Alive
	Elected
)