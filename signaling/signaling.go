// Package signaling implements a small lobby-based HTTP signaling server used
// to establish WebRTC data channel connections between GGPO peers.
//
// A host creates a lobby and receives a lobby ID which it shares out of band
// (e.g. a lobby browser or a chat message). Clients join the lobby, post an
// SDP offer, and poll for the host's SDP answer. The server never inspects
// the session descriptions; it only stores and relays them, so it has no
// WebRTC dependencies of its own.
//
// Endpoints:
//
//	GET  /lobby/host                                      -> lobby ID (text)
//	GET  /lobby/join?id={lobby}                           -> {"ID": playerID}
//	GET  /lobby/delete?id={lobby}
//	GET  /lobby/unregisteredPlayers?id={lobby}            -> [playerID, ...]
//	GET  /offer/get?lobby_id={lobby}&player_id={player}   -> SDP offer
//	POST /offer/post?lobby_id={lobby}&player_id={player}
//	GET  /answer/get?lobby_id={lobby}&player_id={player}  -> SDP answer
//	POST /answer/post?lobby_id={lobby}&player_id={player}
//	GET  /ping                                            -> "pong"
package signaling

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
)

const lobbyIDLength = 6

type clientConnection struct {
	IsHost bool
	Offer  json.RawMessage
	Answer json.RawMessage
}

type lobby struct {
	// host is the first client in clients
	clients []clientConnection
}

// PlayerData is the response body for /lobby/join.
type PlayerData struct {
	// ID is the player's index in the lobby's client list.
	ID int
}

// Server relays session descriptions between peers in lobbies.
type Server struct {
	mutex   sync.Mutex
	lobbies map[string]*lobby
}

func NewServer() *Server {
	return &Server{
		lobbies: make(map[string]*lobby),
	}
}

// Handler returns the http.Handler serving the signaling endpoints. All
// responses allow cross-origin requests so browser builds can use the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/lobby/host", s.lobbyHost)
	mux.HandleFunc("/lobby/join", s.lobbyJoin)
	mux.HandleFunc("/lobby/delete", s.lobbyDelete)
	mux.HandleFunc("/lobby/unregisteredPlayers", s.lobbyUnregisteredPlayers)
	mux.HandleFunc("/offer/get", s.sdpGet(getOffer))
	mux.HandleFunc("/offer/post", s.sdpPost(setOffer))
	mux.HandleFunc("/answer/get", s.sdpGet(getAnswer))
	mux.HandleFunc("/answer/post", s.sdpPost(setAnswer))
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("pong"))
	})
	return allowCORS(mux)
}

// ListenAndServe serves the signaling endpoints on addr, e.g. ":3000".
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) generateLobbyID() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for {
		buffer := make([]rune, lobbyIDLength)
		for i := range buffer {
			buffer[i] = letters[rand.Intn(len(letters))]
		}
		id := string(buffer)
		if _, taken := s.lobbies[id]; !taken {
			return id
		}
	}
}

func (s *Server) lobbyHost(w http.ResponseWriter, _ *http.Request) {
	s.mutex.Lock()
	lobbyID := s.generateLobbyID()
	s.lobbies[lobbyID] = &lobby{
		clients: []clientConnection{{IsHost: true}},
	}
	s.mutex.Unlock()

	w.Write([]byte(lobbyID))
}

func (s *Server) lobbyJoin(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.URL.Query().Get("id")

	s.mutex.Lock()
	defer s.mutex.Unlock()
	l, ok := s.lobbies[lobbyID]
	if !ok {
		http.Error(w, "404 - Lobby not found", http.StatusNotFound)
		return
	}

	l.clients = append(l.clients, clientConnection{IsHost: false})
	pData := PlayerData{ID: len(l.clients) - 1}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pData)
}

func (s *Server) lobbyDelete(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.URL.Query().Get("id")

	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.lobbies, lobbyID)
	w.WriteHeader(http.StatusOK)
}

// lobbyUnregisteredPlayers returns the players the host has not answered yet.
func (s *Server) lobbyUnregisteredPlayers(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.URL.Query().Get("id")

	s.mutex.Lock()
	defer s.mutex.Unlock()
	l, ok := s.lobbies[lobbyID]
	if !ok {
		http.Error(w, "404 - Lobby not found", http.StatusNotFound)
		return
	}

	playerIDs := []int{}
	for i, client := range l.clients {
		if !client.IsHost && client.Answer == nil {
			playerIDs = append(playerIDs, i)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playerIDs)
}

// validatePlayer resolves the lobby and player referenced by the request's
// query parameters. The server mutex must be held by the caller.
func (s *Server) validatePlayer(w http.ResponseWriter, r *http.Request) (*lobby, int, bool) {
	lobbyID := r.URL.Query().Get("lobby_id")
	l, ok := s.lobbies[lobbyID]
	if !ok {
		http.Error(w, "404 - Lobby not found", http.StatusNotFound)
		return nil, 0, false
	}

	playerID, err := strconv.Atoi(r.URL.Query().Get("player_id"))
	if err != nil || playerID < 0 || playerID >= len(l.clients) {
		http.Error(w, "404 - Player not found", http.StatusNotFound)
		return nil, 0, false
	}

	return l, playerID, true
}

func getOffer(c *clientConnection) *json.RawMessage  { return &c.Offer }
func getAnswer(c *clientConnection) *json.RawMessage { return &c.Answer }

func setOffer(c *clientConnection, sdp json.RawMessage)  { c.Offer = sdp }
func setAnswer(c *clientConnection, sdp json.RawMessage) { c.Answer = sdp }

func (s *Server) sdpGet(field func(*clientConnection) *json.RawMessage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		l, playerID, ok := s.validatePlayer(w, r)
		if !ok {
			return
		}
		sdp := *field(&l.clients[playerID])
		if sdp == nil {
			http.Error(w, "404 - Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(sdp)
	}
}

func (s *Server) sdpPost(field func(*clientConnection, json.RawMessage)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sdp json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&sdp); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s.mutex.Lock()
		defer s.mutex.Unlock()
		l, playerID, ok := s.validatePlayer(w, r)
		if !ok {
			return
		}
		field(&l.clients[playerID], sdp)
		w.WriteHeader(http.StatusOK)
	}
}
