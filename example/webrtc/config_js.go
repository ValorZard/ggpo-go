//go:build js && wasm

package main

import (
	"log"
	"syscall/js"
)

// main reads its config from the page URL's query string, so the same wasm
// build serves both roles:
//
//	http://?host=1&lobby=test&signaling=https://sig.example.com -> hosts lobby "test" with signaling server https://sig.example.com
//	http://host/?lobby=test&signaling=https://sig.example.com         -> joins lobby "test" with signaling server https://sig.example.com
//
// An optional &signaling=<url> overrides the signaling server address.
func main() {
	params := js.Global().Get("URLSearchParams").New(
		js.Global().Get("location").Get("search"))

	get := func(key string) string {
		v := params.Call("get", key)
		if v.IsNull() {
			return ""
		}
		return v.String()
	}

	host := get("host")
	err := run(config{
		host:         host == "1" || host == "true" || get("role") == "host",
		lobbyID:      get("lobby"),
		signalingURL: get("signaling"),
	})

	if err != nil {
		log.Fatal(err)
	}
}
