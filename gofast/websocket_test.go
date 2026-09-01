package gofast

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestRegisterWS_EchoesMessage(t *testing.T) {
	router := NewRouter()

	router.RegisterWS("/ws", func(ctx context.Context, conn *websocket.Conn) error {
		var msg string
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return err
		}
		return wsjson.Write(ctx, conn, "echo: "+msg)
	})

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.CloseNow()

	if err := wsjson.Write(ctx, conn, "hello"); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	var reply string
	if err := wsjson.Read(ctx, conn, &reply); err != nil {
		t.Fatalf("failed to read reply: %v", err)
	}

	if reply != "echo: hello" {
		t.Errorf("expected %q, got %q", "echo: hello", reply)
	}

	conn.Close(websocket.StatusNormalClosure, "")
}