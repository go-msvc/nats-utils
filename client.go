package nats

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/go-msvc/errors"
	"github.com/go-msvc/nats-utils/datatype"
	"github.com/go-msvc/utils/ms"
	"github.com/google/uuid"
)

type ClientConfig struct {
	Config
	Async bool `json:"async" doc:"Async client can consume responses for requests sent from other similar clients"`
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
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect")
	}

	//processing of replies
	go func(c *client) {
		for response := range c.replyChan {
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

	//todo: client can respond to either or both:
	//	- responses for own requests
	//	- responses to similar clients (for long running processes)
	//should be an option
	//ms-client is only interested in own responses - sync client
	//workflows would want to get continue much later

	if !newClient.config.Async {
		newClient.config.Domain += "." + uuid.New().String()
	}
	//subscribe to response subject and push them into replyChan for processing above
	if _, err := newClient.conn.subscribe(
		newClient.config.Domain+".response",
		false,
		func(r received) {
			log.Debugf("received:{%s,%s,%+v}", r.subject, r.replySubject, string(r.data))

			//decode into a response
			var response Response
			if err := json.Unmarshal(r.data, &response); err != nil {
				log.Errorf("failed to decode into %T: %s", response, string(r.data))
			} else {
				newClient.replyChan <- response
			}
		},
	); err != nil {
		return nil, errors.Wrapf(err, "failed to subscriber to client replySubject")
	}
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
	resTmpl interface{},
) (
	res interface{},
	err error,
) {
	requestID := uuid.New().String()
	contextID, ok := ctx.Value(ms.CtxID{}).(string)
	if !ok || contextID == "" {
		contextID = requestID
	}
	log.Debugf("Sync config: %+v ctx=%s req=%s", c.config, contextID, requestID)

	//make and register the chan on request id with the client before sending
	//to ensure it exists when response is received
	replyChan := make(chan Response)
	c.pendingMutex.Lock()
	c.pendingReplyChanByRequestID[requestID] = replyChan
	c.pendingMutex.Unlock()

	request := Request{
		Header: Header{
			Timestamp: time.Now(),
			ContextID: contextID,
			RequestID: requestID,
			Client:    &ms.Address{Domain: c.config.Domain, Operation: "todo"},
			Service:   &serviceAddress,
		},
		//Auth: ...? todo
		TTL:  datatype.Duration(ttl),
		Data: req,
	}
	log.Debugf("sending request: %+v", request)
	log.Debugf("sending request.client: %+v", request.Client)
	log.Debugf("sending request.service: %+v", request.Service)

	//now send
	if err := c.conn.sendRequest(request); err != nil {
		return nil, errors.Wrapf(err, "failed to send request")
	}
	log.Debugf("Sent sync request to %+v, waiting %v for response", serviceAddress, ttl)

	//and wait for resply with timeout
	select {
	case response := <-replyChan:
		log.Debugf("Got reply: %+v", response)
		if len(response.Errors) > 0 {
			return nil, ms.NewError(response.Errors...) //failed
		}
		if resTmpl != nil {
			resType := reflect.TypeOf(resTmpl)
			if resType.Kind() == reflect.Ptr {
				resType = resType.Elem()
			}
			//todo: make message struct with this as data then parse JSON only once!
			resPtrValue := reflect.New(resType)
			resJsonData, _ := json.Marshal(response.Data)
			if err := json.Unmarshal(resJsonData, resPtrValue.Interface()); err != nil {
				return nil, errors.Wrapf(err, "cannot parse response data into %v", resType)
			}
			if validator, ok := resPtrValue.Interface().(ms.Validator); ok {
				if err := validator.Validate(); err != nil {
					return nil, errors.Wrapf(err, "invalid response data")
				}
			}
			return resPtrValue.Elem().Interface(), nil
		}
		return response.Data, nil //success
	case <-time.After(ttl):
	}
	log.Errorf("response timeout")
	return nil, errors.Errorf("response timeout")
} //client.Sync()

func (c *client) ASync(addr ms.Address, req interface{}) (err error) {
	return errors.Errorf("NYI")
} //client.ASync()

func (c *client) Send(addr ms.Address, ttl time.Duration, req interface{}) (err error) {
	return errors.Errorf("NYI")
} //client.Send()
