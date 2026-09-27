package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseVlessURI(t *testing.T) {
	uri := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@104.16.1.1:443?type=tcp&security=reality&pbk=123#MyServer"
	node, err := ParseNodeURI(uri, "TestGroup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node.Protocol != "VLESS" {
		t.Errorf("expected VLESS, got %s", node.Protocol)
	}
	if node.Address != "104.16.1.1" {
		t.Errorf("expected 104.16.1.1, got %s", node.Address)
	}
	if node.Port != 443 {
		t.Errorf("expected 443, got %d", node.Port)
	}
	if node.Name != "MyServer" {
		t.Errorf("expected MyServer, got %s", node.Name)
	}
	if node.Security != "reality" {
		t.Errorf("expected reality, got %s", node.Security)
	}
}

func TestParseTrojanURI(t *testing.T) {
	uri := "trojan://password123@1.1.1.1:443?type=ws#TrojanNode"
	node, err := ParseNodeURI(uri, "TestGroup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node.Protocol != "Trojan" {
		t.Errorf("expected Trojan, got %s", node.Protocol)
	}
	if node.Name != "TrojanNode" {
		t.Errorf("expected TrojanNode, got %s", node.Name)
	}
}

func TestParseRemnawaveJSON(t *testing.T) {
	jsonPayload := `[
		{
			"remarks": "🇵🇱 Польша",
			"outbounds": [
				{
					"tag": "proxy",
					"protocol": "vless",
					"settings": {
						"vnext": [
							{
								"address": "pol1.example.com",
								"port": 443,
								"users": [{"id": "uuid-1234", "flow": "xtls-rprx-vision"}]
							}
						]
					},
					"streamSettings": {
						"network": "tcp",
						"security": "reality",
						"realitySettings": {
							"serverName": "pol1.example.com",
							"publicKey": "pubkey-123",
							"shortId": "shortid-123",
							"fingerprint": "firefox"
						}
					}
				},
				{"tag": "direct", "protocol": "freedom"}
			]
		},
		{
			"remarks": "🇩🇪 Германия",
			"outbounds": [
				{
					"tag": "proxy-de",
					"protocol": "shadowsocks",
					"settings": {
						"servers": [
							{
								"address": "de1.example.com",
								"port": 8388,
								"method": "aes-128-gcm",
								"password": "secret-password"
							}
						]
					}
				}
			]
		}
	]`

	nodes, err := ParseContent(jsonPayload, "PandaVPN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	n1 := nodes[0]
	if n1.Name != "🇵🇱 Польша" {
		t.Errorf("expected 🇵🇱 Польша, got %s", n1.Name)
	}
	if n1.Protocol != "VLESS" {
		t.Errorf("expected VLESS, got %s", n1.Protocol)
	}
	if n1.Address != "pol1.example.com" {
		t.Errorf("expected pol1.example.com, got %s", n1.Address)
	}
	if n1.Port != 443 {
		t.Errorf("expected 443, got %d", n1.Port)
	}
	if n1.Security != "reality" {
		t.Errorf("expected reality, got %s", n1.Security)
	}
	if n1.Details["raw_json"] == "" {
		t.Errorf("expected raw_json to be preserved")
	}

	n2 := nodes[1]
	if n2.Name != "🇩🇪 Германия" {
		t.Errorf("expected 🇩🇪 Германия, got %s", n2.Name)
	}
	if n2.Protocol != "Shadowsocks" {
		t.Errorf("expected Shadowsocks, got %s", n2.Protocol)
	}
	if n2.Address != "de1.example.com" {
		t.Errorf("expected de1.example.com, got %s", n2.Address)
	}
	if n2.Port != 8388 {
		t.Errorf("expected 8388, got %d", n2.Port)
	}
}

func TestParseRemnawaveDummyRejection(t *testing.T) {
	dummyJSON := `[
		{
			"remarks": "Приложение не поддерживается",
			"outbounds": [
				{
					"tag": "proxy",
					"protocol": "vless",
					"settings": {
						"vnext": [{"address": "0.0.0.0", "port": 1}]
					}
				}
			]
		}
	]`

	_, err := ParseContent(dummyJSON, "DummySub")
	if err == nil {
		t.Fatalf("expected error for dummy stubs, got nil")
	}
	if !strings.Contains(err.Error(), "provider rejected") {
		t.Errorf("expected provider rejected error, got %v", err)
	}
}

func TestParseClashYAML(t *testing.T) {
	clashYaml := `
proxies:
  - name: "🇺🇸 US Reality"
    type: vless
    server: us1.example.com
    port: 443
    uuid: test-uuid
    network: tcp
    tls: true
    flow: xtls-rprx-vision
    servername: gateway.us.com
    reality-opts:
      public-key: pubkey-test
      short-id: 1a2b
    client-fingerprint: chrome
  - name: "🇯🇵 JP Trojan"
    type: trojan
    server: jp1.example.com
    port: 443
    password: pass
    sni: gateway.jp.com
`
	nodes, err := ParseContent(clashYaml, "ClashSub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	n1 := nodes[0]
	if n1.Name != "🇺🇸 US Reality" {
		t.Errorf("expected 🇺🇸 US Reality, got %s", n1.Name)
	}
	if n1.Protocol != "VLESS" {
		t.Errorf("expected VLESS, got %s", n1.Protocol)
	}
	if n1.Security != "reality" {
		t.Errorf("expected reality, got %s", n1.Security)
	}
	if n1.Details["pbk"] != "pubkey-test" {
		t.Errorf("expected pubkey-test, got %s", n1.Details["pbk"])
	}

	n2 := nodes[1]
	if n2.Name != "🇯🇵 JP Trojan" {
		t.Errorf("expected 🇯🇵 JP Trojan, got %s", n2.Name)
	}
	if n2.Protocol != "Trojan" {
		t.Errorf("expected Trojan, got %s", n2.Protocol)
	}
}

func TestParseBase64Content(t *testing.T) {
	rawURIs := "vless://id@1.1.1.1:443?type=tcp#Node1\nvless://id@2.2.2.2:443?type=tcp#Node2\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(rawURIs))

	nodes, err := ParseContent(encoded, "Base64Sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "Node1" || nodes[1].Name != "Node2" {
		t.Errorf("unexpected node names: %s, %s", nodes[0].Name, nodes[1].Name)
	}
}

func TestLivePandaFetch(t *testing.T) {
	fetcher := NewFetcher("Happ/1.8.0 (Linux; x86_64)")
	sub, err := fetcher.FetchSubscription("https://sub.pandaonline.org/TM6aU3GmVQ6aTdmh", "Panda")
	if err != nil {
		t.Skipf("Live network fetch skipped: %v", err)
		return
	}

	if len(sub.Nodes) == 0 {
		t.Fatalf("expected nodes from Panda subscription, got 0")
	}

	t.Logf("Successfully fetched %d nodes from Panda subscription! First node: %s (%s, %s:%d)",
		len(sub.Nodes), sub.Nodes[0].Name, sub.Nodes[0].Protocol, sub.Nodes[0].Address, sub.Nodes[0].Port)
}

