package app

import (
	"context"
	"encoding/json"

	entitypubsub "github.com/bayu-aditya/ideagate/backend/core/model/entity/pubsub"
	entitywebsocket "github.com/bayu-aditya/ideagate/backend/core/model/entity/websocket"
	"github.com/bayu-aditya/ideagate/backend/core/ports/pubsub"
	"github.com/bayu-aditya/ideagate/backend/core/utils/log"
	"github.com/bayu-aditya/ideagate/backend/server/proxy/usecase"
	"github.com/sirupsen/logrus"
)

func RunSubscriber(pubSub pubsub.IPubSubAdapter, usecaseWebsocketClientManagement usecase.IWebsocketClientManagement) {
	ctx := context.Background()

	subscriber, err := pubSub.Subscribe(ctx, entitypubsub.TopicEventRequest)
	if err != nil {
		log.Fatal(err.Error())
	}

	for eventRequestBytes := range subscriber.Data(ctx) {
		var eventRequest entitywebsocket.Event
		if err = json.Unmarshal(eventRequestBytes, &eventRequest); err != nil {
			logrus.Error("Failed to unmarshal event request", err)
			continue
		}

		client := usecaseWebsocketClientManagement.GetClientByProjectId(ctx, eventRequest.ProjectId)
		if client == nil {
			continue
		}
		client.SendEvent(eventRequest)
	}
}
