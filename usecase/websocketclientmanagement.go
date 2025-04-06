package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	entitypubsub "github.com/ideagate/core/model/entity/pubsub"
	entitywebsocket "github.com/ideagate/core/model/entity/websocket"
	"github.com/ideagate/core/ports/distributionlock"
	"github.com/ideagate/core/ports/pubsub"
	"github.com/ideagate/core/utils/log"
	"github.com/sirupsen/logrus"
)

type IWebsocketClientManagement interface {
	NewClient(ctx context.Context, conn *websocket.Conn, projectId, controllerId string) IWebsocketClient
	GetClientByProjectId(ctx context.Context, projectId string) IWebsocketClient
}

type IWebsocketClient interface {
	SendEvent(event entitywebsocket.Event)
	WorkerReceiveEvent()
	WaitConnectionClose() <-chan struct{}
}

func NewWebsocketClientManagement(
	pubSub pubsub.IPubSubAdapter,
	distributedLock distributionlock.IDistributionLock,
) IWebsocketClientManagement {
	return &WebsocketClientManagement{
		clients:         &sync.Map{},
		pubSub:          pubSub,
		distributedLock: distributedLock,
	}
}

type WebsocketClientManagement struct {
	clients         *sync.Map // map[projectID]map[controllerID]WebsocketClient
	pubSub          pubsub.IPubSubAdapter
	distributedLock distributionlock.IDistributionLock
}

func (m *WebsocketClientManagement) NewClient(ctx context.Context, conn *websocket.Conn, projectId, controllerId string) IWebsocketClient {
	newClient := NewWebsocketClient(ctx, conn, projectId, controllerId, m.deleteClient, m.pubSub, m.distributedLock)

	projectClients, isFound := m.clients.Load(projectId)
	if !isFound {
		projectClients = &sync.Map{}
	}

	projectClientsMap := projectClients.(*sync.Map)
	projectClientsMap.Store(controllerId, newClient)

	m.clients.Store(projectId, projectClients)

	return newClient
}

func (m *WebsocketClientManagement) GetClientByProjectId(_ context.Context, projectId string) IWebsocketClient {
	projectClients, isFound := m.clients.Load(projectId)
	if !isFound {
		return nil
	}

	var client *WebsocketClient

	projectClientsMap := projectClients.(*sync.Map)
	projectClientsMap.Range(func(_, wsClient any) bool {
		client = wsClient.(*WebsocketClient)
		return false
	})

	return client
}

func (m *WebsocketClientManagement) deleteClient(projectId, controllerId string) {
	projectClients, isFound := m.clients.Load(projectId)
	if !isFound {
		return
	}

	projectClientsMap := projectClients.(*sync.Map)
	projectClientsMap.Delete(controllerId)
}

func NewWebsocketClient(
	ctx context.Context,
	conn *websocket.Conn,
	projectId,
	controllerId string,
	deleteClientFunc func(projectId, controllerId string),
	pubSub pubsub.IPubSubAdapter,
	distributedLock distributionlock.IDistributionLock,
) IWebsocketClient {
	ctx, cancelFunc := context.WithCancel(ctx)

	return &WebsocketClient{
		ctx:              ctx,
		cancelCtxFunc:    cancelFunc,
		conn:             conn,
		projectId:        projectId,
		connectionId:     controllerId,
		deleteClientFunc: deleteClientFunc,
		pubSub:           pubSub,
		distributedLock:  distributedLock,
	}
}

type WebsocketClient struct {
	ctx              context.Context
	cancelCtxFunc    context.CancelFunc
	conn             *websocket.Conn
	projectId        string
	connectionId     string
	deleteClientFunc func(projectId, controllerId string)
	pubSub           pubsub.IPubSubAdapter
	distributedLock  distributionlock.IDistributionLock
}

func (c *WebsocketClient) SendEvent(event entitywebsocket.Event) {
	// check idempotency by project id and event id
	lockKey := fmt.Sprintf("%s:%s", c.projectId, event.Id)
	isAllow, err := c.distributedLock.Lock(c.ctx, lockKey)
	if err != nil {
		log.Error("distributed lock: %v", err)
		c.cancelCtxFunc()
		return
	}
	if !isAllow {
		return
	}
	_ = c.distributedLock.Unlock(context.Background(), lockKey)

	// send to websocket client
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Error("marshal event: %v", err)
		c.cancelCtxFunc()
		return
	}

	if err = c.conn.WriteMessage(websocket.TextMessage, eventBytes); err != nil {
		logrus.Error("write message", err)
		c.cancelCtxFunc()
	}
}

func (c *WebsocketClient) WorkerReceiveEvent() {
	for {
		// read event from websocket client
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			logrus.Error("read message", err)
			c.cancelCtxFunc()
			break
		}

		// publish event to PubSub distributed, if error only log it
		if err = c.pubSub.Publish(context.Background(), entitypubsub.TopicEventResponse, message); err != nil {
			log.Error("publish: %v", err)
		}
	}
}

func (c *WebsocketClient) WaitConnectionClose() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		<-c.ctx.Done()
		c.deleteClientFunc(c.projectId, c.connectionId)
		close(ch)
	}()
	return ch
}
