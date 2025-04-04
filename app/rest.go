package app

import (
	"encoding/json"
	"net/http"
	"time"

	entitypubsub "github.com/bayu-aditya/ideagate/backend/core/model/entity/pubsub"
	entitywebsocket "github.com/bayu-aditya/ideagate/backend/core/model/entity/websocket"
	"github.com/bayu-aditya/ideagate/backend/core/ports/pubsub"
	"github.com/bayu-aditya/ideagate/backend/server/proxy/usecase"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	addr       = ":8080"
	wsUpgrader = websocket.Upgrader{}
)

func RunRestHttp(pubSub pubsub.IPubSubAdapter, usecaseWebsocketClientManagement usecase.IWebsocketClientManagement) {
	http.HandleFunc("/", healthCheckHandler)
	http.HandleFunc("/event/send", sendEventHandler(pubSub))
	http.HandleFunc("/event/ws", websocketProxyHandler(usecaseWebsocketClientManagement))

	logrus.Infof("Proxy server running on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logrus.Fatal(err)
	}
}

func healthCheckHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func sendEventHandler(pubSub pubsub.IPubSubAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()

		requestBody := struct {
			ProjectId string `json:"project_id"`
			Type      string `json:"type"`
			Data      any    `json:"data"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		if requestBody.ProjectId == "" {
			http.Error(w, "Project ID is required", http.StatusBadRequest)
			return
		}

		// construct event
		eventRequest := entitywebsocket.Event{
			Id:        uuid.NewString(),
			ProjectId: requestBody.ProjectId,
			Type:      requestBody.Type,
			Data:      requestBody.Data,
		}

		eventRequestBytes, err := json.Marshal(eventRequest)
		if err != nil {
			http.Error(w, "Failed to marshal event", http.StatusInternalServerError)
			return
		}

		// subscribe event from topic event response
		subscriber, err := pubSub.Subscribe(ctx, entitypubsub.TopicEventResponse)
		if err != nil {
			http.Error(w, "Failed to subscribe event", http.StatusInternalServerError)
			return
		}
		defer subscriber.Close()

		// publish event to topic event request
		if err = pubSub.Publish(ctx, entitypubsub.TopicEventRequest, eventRequestBytes); err != nil {
			http.Error(w, "Failed to publish event", http.StatusInternalServerError)
			return
		}

		timeoutChan := time.After(10 * time.Second)
		for {
			select {
			case <-timeoutChan:
				http.Error(w, "Request timed out", http.StatusGatewayTimeout)
				return

			case responseData := <-subscriber.Data(ctx):
				eventResponse := entitywebsocket.Event{}
				if err = json.Unmarshal(responseData, &eventResponse); err != nil {
					logrus.Error("unmarshal event response", err)
					continue
				}
				if eventResponse.Id != eventRequest.Id {
					continue
				}

				responseBytes, err := json.Marshal(eventResponse)
				if err != nil {
					http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(responseBytes)
				return
			}
		}
	}
}

func websocketProxyHandler(usecaseWebsocketClientManagement usecase.IWebsocketClientManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		projectID := r.URL.Query().Get("project_id")
		if projectID == "" {
			http.Error(w, "Project ID is required", http.StatusBadRequest)
			return
		}

		connectionID := r.Header.Get("connection_id")
		if connectionID == "" {
			http.Error(w, "Connection ID is required", http.StatusBadRequest)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.Error(err)
			return
		}
		defer conn.Close()

		websocketClient := usecaseWebsocketClientManagement.NewClient(ctx, conn, projectID, connectionID)
		go websocketClient.WorkerReceiveEvent()
		<-websocketClient.WaitConnectionClose()
	}
}
