//go:build js && wasm

package main

import "syscall/js"

// main reads its config from the page URL's query string, so the same wasm
// build serves both roles:
//
//	http://host/?host=1&lobby=test   -> hosts lobby "test"
//	http://host/?lobby=test          -> joins lobby "test"
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
	run(config{
		host:         host == "1" || host == "true" || get("role") == "host",
		lobbyID:      get("lobby"),
		signalingURL: get("signaling"),
	})
}
