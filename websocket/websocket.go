package websocket

import (
	"encoding/json"
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/buger/jsonparser"
	"github.com/gorilla/websocket"
	"time"
)

const (
	ApiKey       = ""
	ApiSecret    = ""
	url          = "wss://paper-api.alpaca.markets/stream"
	tradeUpdates = "trade_updates"
)

func subscribeMessages() [][]byte {
	// auth
	auth, _ := json.Marshal(Auth{
		Action: "authenticate",
		Data: AuthData{
			KeyId:     ApiKey,
			SecretKey: ApiSecret,
		},
	})
	// listen
	listen, _ := json.Marshal(Listen{
		Action: "listen",
		Data: ListenData{
			Streams: []string{tradeUpdates},
		},
	})
	messages := [][]byte{auth, listen}
	return messages
}

func handle(payload []byte, tradeUpdateChannels map[string]chan alpaca.TradeUpdate) {
	stream, _ := jsonparser.GetString(payload, "stream")
	if stream != tradeUpdates {
		fmt.Println("Received: ", string(payload))
		return
	}
	data, _, _, _ := jsonparser.Get(payload, "data")
	var tradeUpdate alpaca.TradeUpdate
	if err := json.Unmarshal(data, &tradeUpdate); err != nil {
		return
	}

	ch := tradeUpdateChannels[tradeUpdate.Order.Symbol]
	ch <- tradeUpdate
}

func connectAndSubscribe() (*websocket.Conn, error) {
	// connect ws
	fmt.Println("connecting to private websocket...")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	// write initial payloads (auth + topic subscription)
	payloads := subscribeMessages()
	if payloads != nil && len(payloads) > 0 {
		for _, payload := range payloads {
			err = conn.WriteMessage(websocket.BinaryMessage, payload)
			if err != nil {
				return nil, err
			}
		}
	}

	return conn, nil
}

func process(conn *websocket.Conn, tradeUpdateChannels map[string]chan alpaca.TradeUpdate) {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)

	for {
		err := conn.SetReadDeadline(time.Now().Add(time.Duration(300) * time.Second))
		if err != nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			conn2, err := connectAndSubscribe()
			if err != nil {
				fmt.Println(err)
				fmt.Println("Can't reconnect!!")
				return
			}
			go process(conn2, tradeUpdateChannels)
			return
		}

		handle(msg, tradeUpdateChannels)
	}
}

func AlpacaPrivateStream(tradeUpdateChannels map[string]chan alpaca.TradeUpdate) error {
	// connect ws
	conn, err := connectAndSubscribe()
	if err != nil {
		return err
	}

	go process(conn, tradeUpdateChannels)

	return nil
}
