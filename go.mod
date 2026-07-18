module stell/coding-agent

go 1.25.0

require (
	github.com/atotto/clipboard v0.1.4
	github.com/mattn/go-runewidth v0.0.16
	golang.org/x/mod v0.38.0
	stell/agent v0.0.0
	github.com/stelmakhdigital/ai v0.0.0
	stell/tui v0.0.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
)

replace github.com/stelmakhdigital/ai => ../ai

replace stell/agent => ../agent

replace stell/tui => ../tui
