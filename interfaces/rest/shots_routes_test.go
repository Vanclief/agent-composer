package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// testPNG encodes a small PNG of one color.
func testPNG(t *testing.T, shade uint8) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for x := 0; x < 4; x++ {
		for y := 0; y < 3; y++ {
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}

	out := &bytes.Buffer{}
	err := png.Encode(out, img)
	if err != nil {
		t.Fatal(err)
	}

	return out.Bytes()
}

// deliverBody is a deliver request with a report naming shots and files
// holding the PNGs, as JSON.
func deliverBody(t *testing.T, shots []taskmodels.Shot, files map[string][]byte) string {
	t.Helper()

	body := map[string]any{
		"summary": "Filled the icon.",
		"report": map[string]any{
			"criteria": []map[string]string{{"met": "yes"}},
			"shots":    shots,
		},
		"files": files,
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// ADR 0010 "Delivery report", shots: a delivery's PNGs are kept under the
// agc home, served at /api/tasks/{name}/shots/{label}.png, listed with
// their URLs on the task and in the Delivered message; a non-PNG, an
// oversize file or a missing one is refused.
func TestTaskDeliverShotsRoutes(t *testing.T) {
	t.Setenv(tasks.EnvPublicURL, "https://agc.example.com/")

	rt := &statefulRuntime{sb: &fakeSandbox{}, statuses: map[string]string{}}
	routes := newTaskRoutes(t, rt, "")

	before := testPNG(t, 10)
	after := testPNG(t, 200)
	shots := []taskmodels.Shot{{Label: "before", File: "/tmp/before.png"}, {Label: "after", File: "/tmp/after.png"}}

	big := append(append([]byte{}, before...), make([]byte, taskmodels.MaxShotBytes)...)

	routes.check(t, []routeCase{
		{http.MethodPost, "/api/tasks", `{"title":"Stop icon","brief":"b","manager":"orchestrator","accept":["the icon is filled"]}`, http.StatusOK, `"report":null`},
		{http.MethodGet, "/api/tasks/stop-icon/shots/before.png", "", http.StatusNotFound, "task stop-icon has no shot before"},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, shots, map[string][]byte{"before": before, "after": []byte("GIF89a not a png")}), http.StatusBadRequest, `shot \"after\" is not a PNG`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, shots, map[string][]byte{"before": before, "after": big}), http.StatusBadRequest, `bytes, at most 2097152 (2 MB)`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, shots, map[string][]byte{"before": before}), http.StatusBadRequest, `shot \"after\" has no file`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, []taskmodels.Shot{{Label: "before", File: "before.jpg"}}, map[string][]byte{"before": before}), http.StatusBadRequest, "a shot is a PNG file"},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, []taskmodels.Shot{{Label: "Before", File: "b.png"}}, map[string][]byte{"Before": before}), http.StatusBadRequest, "use a slug"},
		{http.MethodGet, "/api/tasks/stop-icon", "", http.StatusOK, `"delivered_at":null`},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", deliverBody(t, shots, map[string][]byte{"before": before, "after": after}), http.StatusOK, `"shots":[{"label":"before","file":"before.png","url":"https://agc.example.com/api/tasks/stop-icon/shots/before.png"},{"label":"after","file":"after.png","url":"https://agc.example.com/api/tasks/stop-icon/shots/after.png"}]`},
		{http.MethodGet, "/api/msg/read?channel=%23stop-icon", "", http.StatusOK, `\n\nBefore: https://agc.example.com/api/tasks/stop-icon/shots/before.png\nAfter: https://agc.example.com/api/tasks/stop-icon/shots/after.png"`},
		{http.MethodGet, "/api/tasks/stop-icon/shots/after.jpg", "", http.StatusNotFound, "shots are served as"},
		{http.MethodGet, "/api/tasks/stop-icon/shots/side.png", "", http.StatusNotFound, "task stop-icon has no shot side"},
		{http.MethodGet, "/api/tasks/stop-icon/shots/..%2Fx.png", "", http.StatusNotFound, "no shot"},
		{http.MethodGet, "/api/tasks/nope/shots/before.png", "", http.StatusNotFound, "task nope not found"},
	})

	for label, want := range map[string][]byte{"before": before, "after": after} {
		recorder := httptest.NewRecorder()
		routes.e.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/tasks/stop-icon/shots/"+label+".png", nil))

		if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" || !bytes.Equal(recorder.Body.Bytes(), want) {
			t.Fatalf("%s: status %d, type %q, %d bytes", label, recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Len())
		}
	}

	// A reject keeps the shots; a delivery without them removes them.
	routes.check(t, []routeCase{
		{http.MethodPost, "/api/tasks/stop-icon/reject", `{"by":"orchestrator","why":"too small"}`, http.StatusOK, `"label":"before"`},
		{http.MethodGet, "/api/tasks/stop-icon/shots/before.png", "", http.StatusOK, ""},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", `{"summary":"Again.","files":{"after":"` + base64.StdEncoding.EncodeToString(after) + `"}}`, http.StatusBadRequest, "the report has no shots"},
		{http.MethodPost, "/api/tasks/stop-icon/deliver", `{"summary":"Again, no shots."}`, http.StatusOK, `"report":null`},
		{http.MethodGet, "/api/tasks/stop-icon/shots/before.png", "", http.StatusNotFound, "has no shot before"},
	})
}
