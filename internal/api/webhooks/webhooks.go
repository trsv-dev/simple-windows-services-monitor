package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Webhook struct {
}

func NewWebhook() *Webhook {
	return &Webhook{}
}

type KeycloakEvent struct {
	OperationType string `json:"operationType"` // "DELETE"
	ResourceType  string `json:"resourceType"`  // "USER"
	ResourcePath  string `json:"resourcePath"`  // "users/xxx-xxx-xxx"
	RealmID       string `json:"realmId"`       // "swsm"
	Time          int64  `json:"time"`
}

func (wh *Webhook) DeleteUserWebhook(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("%+v\n", r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Can't read body", http.StatusBadRequest)
		return
	}

	var event KeycloakEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Can't parse JSON", http.StatusBadRequest)
		return
	}

	// 2. Process the event (e.g., sync user data, trigger workflow)
	log.Printf("Received event Type: %s for Realm ID: %s", event.OperationType, event.RealmID)

	// 3. Send a success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event received and processed"))
}
