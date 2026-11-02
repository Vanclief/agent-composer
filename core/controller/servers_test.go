package controller

import (
	"strings"
	"testing"
)

func TestValidatePeers(t *testing.T) {
	peers, err := ValidatePeers([]ServerSettings{{Name: "mini", URL: " http://192.168.1.20:1202/ "}, {Name: "attic", URL: "https://attic.lan"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 || peers[0].Name != "attic" || peers[1].URL != "http://192.168.1.20:1202" {
		t.Fatalf("peers = %+v", peers)
	}

	cases := []struct {
		servers []ServerSettings
		want    string
	}{
		{[]ServerSettings{{Name: "local", URL: "http://a"}}, "local is this server"},
		{[]ServerSettings{{Name: "Mini", URL: "http://a"}}, "lowercase"},
		{[]ServerSettings{{Name: "mini", URL: "http://a"}, {Name: "mini", URL: "http://b"}}, "listed twice"},
		{[]ServerSettings{{Name: "mini", URL: "192.168.1.20:1202"}}, "must be http:// or https://"},
		{[]ServerSettings{{Name: "mini", URL: ""}}, "must be http:// or https://"},
	}
	for _, tc := range cases {
		_, err := ValidatePeers(tc.servers)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%+v: %v, want %q", tc.servers, err, tc.want)
		}
	}
}

func TestListenAddress(t *testing.T) {
	ctrl := &Controller{Config: Config{App: AppSettings{Port: "1202"}}}
	if ctrl.ListenAddress() != ":1202" {
		t.Fatalf("default = %q, want :1202 as before listen:", ctrl.ListenAddress())
	}

	ctrl.Config.Listen = "127.0.0.1:1202"
	if ctrl.ListenAddress() != "127.0.0.1:1202" {
		t.Fatalf("listen = %q", ctrl.ListenAddress())
	}
}
