package app

import (
	"context"
	"encoding/json"

	entitypubsub "github.com/ideagate/core/model/entity/pubsub"
	entitywebsocket "github.com/ideagate/core/model/entity/websocket"
	"github.com/ideagate/core/ports/pubsub"
	"github.com/ideagate/core/utils/log"
	"github.com/ideagate/server-proxy/usecase"
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
