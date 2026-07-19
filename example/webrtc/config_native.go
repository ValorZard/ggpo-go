//go:build !js || !wasm

package main

import "flag"

func main() {
	host := flag.Bool("host", false, "host the lobby (otherwise join it)")
	lobby := flag.String("lobby", "ggpo-test", "shared lobby id both players agree on")
	signaling := flag.String("signaling", "http://localhost:3000", "signaling server url")
	flag.Parse()

	run(config{
		host:         *host,
		lobbyID:      *lobby,
		signalingURL: *signaling,
	})
}
