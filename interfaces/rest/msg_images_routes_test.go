package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// postBody is a post request as JSON, images base64.
func postBody(t *testing.T, participant string, body string, images []msgmodels.ImageUpload) string {
	t.Helper()

	data, err := json.Marshal(map[string]any{"participant_id": participant, "channel": "#talk", "body": body, "images": images})
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// ADR 0005 "Images": a post carries up to four images, kept under the agc
// home, listed on the message with their URLs and served at
// /api/msg/images/{message id}/{name}; anything else is refused, and a
// text-only post is as before.
func TestMsgImagesRoutes(t *testing.T) {
	routes := newTaskRoutes(t, &fakeRuntime{sb: &fakeSandbox{}}, "")

	status, body := routes.do(http.MethodPost, "/api/msg/join", `{"name":"franco"}`)
	if status != http.StatusOK {
		t.Fatalf("join: status %d, body %s", status, body)
	}

	var franco msgmodels.Participant
	err := json.Unmarshal([]byte(body), &franco)
	if err != nil {
		t.Fatal(err)
	}

	shot := testPNG(t, 30)
	other := testPNG(t, 220)
	big := append(append([]byte{}, shot...), make([]byte, msgmodels.MaxImageBytes)...)
	five := []msgmodels.ImageUpload{{Name: "a.png", Data: shot}, {Name: "b.png", Data: shot}, {Name: "c.png", Data: shot}, {Name: "d.png", Data: shot}, {Name: "e.png", Data: shot}}

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "plain", nil), http.StatusOK, `"body":"plain","kind":"message","created_at"`},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "", nil), http.StatusBadRequest, "body is required"},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "x", five), http.StatusBadRequest, "at most 4 images, this one has 5"},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "x", []msgmodels.ImageUpload{{Name: "big.png", Data: big}}), http.StatusBadRequest, "(5 MB)"},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "x", []msgmodels.ImageUpload{{Name: "notes.txt", Data: []byte("hello there")}}), http.StatusBadRequest, "send a PNG, JPEG, GIF or WebP"},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "look", []msgmodels.ImageUpload{{Name: "../Screen Shot.png", Data: shot}, {Name: "Screen Shot.png", Data: other}}), http.StatusOK,
			`"images":[{"name":"Screen-Shot.png","type":"image/png","size":` + itoa(len(shot)) + `,"url":"/api/msg/images/2/Screen-Shot.png"},{"name":"Screen-Shot-2.png","type":"image/png","size":` + itoa(len(other)) + `,"url":"/api/msg/images/2/Screen-Shot-2.png"}]`},
		{http.MethodPost, "/api/msg/post", postBody(t, franco.ID, "", []msgmodels.ImageUpload{{Name: "only.png", Data: shot}}), http.StatusOK, `"body":"","kind":"message","images":[{"name":"only.png"`},
		{http.MethodGet, "/api/msg/read?channel=%23talk", "", http.StatusOK, `"url":"/api/msg/images/2/Screen-Shot-2.png"`},
		{http.MethodGet, "/api/msg/images/2/nope.png", "", http.StatusNotFound, "message 2 has no image nope.png"},
		{http.MethodGet, "/api/msg/images/1/x.png", "", http.StatusNotFound, "message 1 has no image x.png"},
		{http.MethodGet, "/api/msg/images/99/x.png", "", http.StatusNotFound, "message 99 has no image"},
		{http.MethodGet, "/api/msg/images/abc/x.png", "", http.StatusNotFound, "no message abc"},
		{http.MethodGet, "/api/msg/images/2/..%2F..%2Fsecrets.env", "", http.StatusNotFound, "has no image"},
	})

	status, body = routes.do(http.MethodGet, "/api/msg/read?channel=%23talk", "")
	if status != http.StatusOK || strings.Contains(body, `"body":"plain","kind":"message","images"`) {
		t.Fatalf("a text-only message lists no images: %s", body)
	}

	for name, want := range map[string][]byte{"Screen-Shot.png": shot, "Screen-Shot-2.png": other} {
		recorder := httptest.NewRecorder()
		routes.e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/msg/images/2/"+name, nil))

		if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" || !bytes.Equal(recorder.Body.Bytes(), want) {
			t.Fatalf("%s: status %d, type %q, %d bytes", name, recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Len())
		}
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
