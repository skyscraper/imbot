package websocket

type AuthData struct {
	KeyId     string `json:"key_id"`
	SecretKey string `json:"secret_key"`
}

type Auth struct {
	Action string   `json:"action"`
	Data   AuthData `json:"data"`
}

type ListenData struct {
	Streams []string `json:"streams"`
}

type Listen struct {
	Action string     `json:"action"`
	Data   ListenData `json:"data"`
}
