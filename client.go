package nats

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-msvc/errors"
	"github.com/go-msvc/nats-utils/datatype"
	"github.com/go-msvc/utils/ms"
	"github.com/google/uuid"
)

type ClientConfig struct {
	Config
}

func (clientConfig ClientConfig) Create() (ms.Client, error) {
	newClient := &client{
		config:                      clientConfig,
		XreplySubject:               clientConfig.Domain + "." + uuid.New().String() + ".response",
		replyChan:                   make(chan Response),
		pendingReplyChanByRequestID: map[string]chan Response{},
	}

	var err error
	log.Debugf("connecting...")
	newClient.conn, err = connect(clientConfig.Config)
	log.Debugf("1")
	if err != nil {
		log.Debugf("1")
		return nil, errors.Wrapf(err, "failed to connect")
	}

	//processing of replies
	log.Debugf("1")
	go func(c *client) {
		log.Debugf("1")
		for response := range c.replyChan {
			log.Debugf("1")
			log.Debugf("received response: %+v", response)

			c.pendingMutex.Lock()
			pendingChan, ok := c.pendingReplyChanByRequestID[response.RequestID]
			c.pendingMutex.Unlock()
			if !ok {
				log.Errorf("discard unexpected response: %+v", response)
			} else {
				//this push may never block - ensure that these channels are served well!
				pendingChan <- response

				c.pendingMutex.Lock()
				close(pendingChan)
				delete(c.pendingReplyChanByRequestID, response.RequestID)
				c.pendingMutex.Unlock()
			}
		} //for each response
		log.Debugf("Stopped reading responses")
	}(newClient)

	//client can respond to either or both:
	//	- responses for own requests
	//	- responses to similar clients (for long running processes)
	...

	log.Debugf("1")
	//subscribe to response subject and push them into replyChan for processing above
	if _, err := newClient.conn.subscribe(
		newClient.config.Domain+".response",
		false,
		func(r received) {
			log.Debugf("received: %+v", r)

			//decode into a response
			var response Response
			if err := json.Unmarshal(r.data, &response); err != nil {
				log.Errorf("failed to decode into %T: %s", response, string(r.data))
			} else {
				newClient.replyChan <- response
			}
		},
	); err != nil {
		log.Debugf("1")
		return nil, errors.Wrapf(err, "failed to subscriber to client replySubject")
	}
	log.Debugf("1")

	//todo: cleanup above when stopping
	return newClient, nil
} //ClientConfig.Create()

type client struct {
	config                      ClientConfig
	conn                        *connection
	XreplySubject               string
	replyChan                   chan Response
	pendingMutex                sync.Mutex
	pendingReplyChanByRequestID map[string]chan Response
}

func (c *client) Sync(
	ctx context.Context,
	serviceAddress ms.Address,
	ttl time.Duration,
	req interface{},
) (
	res interface{},
	err error,
) {
	requestID := uuid.New().String()
	contextID, ok := ctx.Value(ms.CtxID{}).(string)
	if !ok || contextID == "" {
		contextID = requestID
	}

	log.Debugf("1")

	//make and register the chan on request id with the client before sending
	//to ensure it exists when response is received
	replyChan := make(chan Response)
	log.Debugf("1")
	c.pendingMutex.Lock()
	log.Debugf("1")
	c.pendingReplyChanByRequestID[requestID] = replyChan
	log.Debugf("1")
	c.pendingMutex.Unlock()
	log.Debugf("1")

	request := Request{
		Header: Header{
			Timestamp: time.Now(),
			ContextID: contextID,
			RequestID: requestID,
			//client domain will be used by server for reply path, and must be unique
			//in case other clients are used at the same time! And the client must be
			//subscribed to this topic
			Client:  &ms.Address{Domain: c.config.Domain + "." + contextID, Operation: "todo"},
			Service: &serviceAddress,
		},
		//Auth: ...? todo
		TTL:  datatype.Duration(ttl),
		Data: req,
	}

	//now send
	log.Debugf("1")
	if err := c.conn.sendRequest(request); err != nil {
		log.Debugf("1")
		return nil, errors.Wrapf(err, "failed to send request")
	}
	log.Debugf("Sent sync request to %+v, waiting %v for response", serviceAddress, ttl)

	//and wait for resply with timeout
	select {
	case response := <-replyChan:
		log.Debugf("Got reply: %+v", response)
		return response.Data, nil
	case <-time.After(ttl):
	}
	log.Errorf("response timeout")
	return nil, errors.Errorf("response timeout")
} //Sync()

func (c *client) ASync(addr ms.Address, req interface{}) (err error) {
	log.Debugf("1")
	return errors.Errorf("NYI")
}

func (c *client) Send(addr ms.Address, ttl time.Duration, req interface{}) (err error) {
	log.Debugf("1")
	return errors.Errorf("NYI")
}
