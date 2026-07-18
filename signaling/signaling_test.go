package signaling_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ikemen-engine/ggpo/signaling"
)

func get(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %s", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s failed: %s", url, err)
	}
	return resp.StatusCode, body
}

func post(t *testing.T, url string, payload []byte) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s failed: %s", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestSignalingLobbyFlow(t *testing.T) {
	server := httptest.NewServer(signaling.NewServer().Handler())
	defer server.Close()

	// Host a lobby.
	status, lobbyID := get(t, server.URL+"/lobby/host")
	if status != http.StatusOK {
		t.Fatalf("hosting lobby returned status %d", status)
	}
	if len(lobbyID) == 0 {
		t.Fatalf("hosting lobby returned an empty lobby ID")
	}

	// Joining a nonexistent lobby fails.
	status, _ = get(t, server.URL+"/lobby/join?id=doesNotExist")
	if status != http.StatusNotFound {
		t.Errorf("joining a nonexistent lobby returned status %d, want 404", status)
	}

	// Join the real lobby; the host occupies index 0 so the first joiner is 1.
	status, body := get(t, server.URL+"/lobby/join?id="+string(lobbyID))
	if status != http.StatusOK {
		t.Fatalf("joining lobby returned status %d", status)
	}
	var pData signaling.PlayerData
	if err := json.Unmarshal(body, &pData); err != nil {
		t.Fatalf("decoding join response failed: %s", err)
	}
	if pData.ID != 1 {
		t.Errorf("first joiner got player ID %d, want 1", pData.ID)
	}

	// The joiner shows up as unregistered until the host posts an answer.
	status, body = get(t, server.URL+"/lobby/unregisteredPlayers?id="+string(lobbyID))
	if status != http.StatusOK {
		t.Fatalf("unregisteredPlayers returned status %d", status)
	}
	var playerIDs []int
	if err := json.Unmarshal(body, &playerIDs); err != nil {
		t.Fatalf("decoding unregisteredPlayers failed: %s", err)
	}
	if len(playerIDs) != 1 || playerIDs[0] != 1 {
		t.Errorf("unregisteredPlayers = %v, want [1]", playerIDs)
	}

	player := "?lobby_id=" + string(lobbyID) + "&player_id=1"

	// No offer posted yet.
	status, _ = get(t, server.URL+"/offer/get"+player)
	if status != http.StatusNotFound {
		t.Errorf("getting a missing offer returned status %d, want 404", status)
	}

	// Post and read back an offer.
	offer := []byte(`{"type":"offer","sdp":"v=0 fake"}`)
	if status := post(t, server.URL+"/offer/post"+player, offer); status != http.StatusOK {
		t.Fatalf("posting offer returned status %d", status)
	}
	status, body = get(t, server.URL+"/offer/get"+player)
	if status != http.StatusOK {
		t.Fatalf("getting offer returned status %d", status)
	}
	if !bytes.Equal(bytes.TrimSpace(body), offer) {
		t.Errorf("got offer %s, want %s", body, offer)
	}

	// Post and read back an answer.
	answer := []byte(`{"type":"answer","sdp":"v=0 fake"}`)
	if status := post(t, server.URL+"/answer/post"+player, answer); status != http.StatusOK {
		t.Fatalf("posting answer returned status %d", status)
	}
	status, body = get(t, server.URL+"/answer/get"+player)
	if status != http.StatusOK {
		t.Fatalf("getting answer returned status %d", status)
	}
	if !bytes.Equal(bytes.TrimSpace(body), answer) {
		t.Errorf("got answer %s, want %s", body, answer)
	}

	// Once answered, the player is registered.
	status, body = get(t, server.URL+"/lobby/unregisteredPlayers?id="+string(lobbyID))
	if status != http.StatusOK {
		t.Fatalf("unregisteredPlayers returned status %d", status)
	}
	if err := json.Unmarshal(body, &playerIDs); err != nil {
		t.Fatalf("decoding unregisteredPlayers failed: %s", err)
	}
	if len(playerIDs) != 0 {
		t.Errorf("unregisteredPlayers after answer = %v, want []", playerIDs)
	}

	// Delete the lobby; joining it afterwards fails.
	status, _ = get(t, server.URL+"/lobby/delete?id="+string(lobbyID))
	if status != http.StatusOK {
		t.Fatalf("deleting lobby returned status %d", status)
	}
	status, _ = get(t, server.URL+"/lobby/join?id="+string(lobbyID))
	if status != http.StatusNotFound {
		t.Errorf("joining a deleted lobby returned status %d, want 404", status)
	}
}

func TestSignalingInvalidPlayer(t *testing.T) {
	server := httptest.NewServer(signaling.NewServer().Handler())
	defer server.Close()

	_, lobbyID := get(t, server.URL+"/lobby/host")

	status, _ := get(t, server.URL+"/offer/get?lobby_id="+string(lobbyID)+"&player_id=99")
	if status != http.StatusNotFound {
		t.Errorf("out of range player returned status %d, want 404", status)
	}
	status, _ = get(t, server.URL+"/offer/get?lobby_id="+string(lobbyID)+"&player_id=notANumber")
	if status != http.StatusNotFound {
		t.Errorf("non-numeric player returned status %d, want 404", status)
	}
	status, _ = get(t, server.URL+"/offer/get?lobby_id=nope&player_id=1")
	if status != http.StatusNotFound {
		t.Errorf("missing lobby returned status %d, want 404", status)
	}
}
