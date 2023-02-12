package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/go-msvc/errors"
	"github.com/go-msvc/utils/ms"
	"github.com/nats-io/nats.go"
)

type ServerConfig struct {
	Config
}

func (c ServerConfig) Create() (ms.Server, error) {
	return &server{
		config: c,
		svc:    nil, //defined in Serve()
	}, nil
}

type server struct {
	config             ServerConfig
	conn               *connection
	svc                ms.MicroService
	replySubjectPrefix string
	replySubscription  *nats.Subscription
	replyChannelsLock  sync.Mutex
	replyChannels      map[string]chan received
	client             ms.Client
}

func (s *server) Serve(svc ms.MicroService) error {
	s.svc = svc
	s.replyChannels = make(map[string]chan received, 100)
	s.replySubjectPrefix = nats.NewInbox()

	var err error
	s.conn, err = connect(s.config.Config)
	if err != nil {
		return errors.Wrapf(err, "server failed to connect to nats")
	}

	if _, err := s.conn.subscribe(s.config.Domain+".request", false /*, h.handleRequest*/, s.handleRequest); err != nil {
		return errors.Wrapf(err, "failed to subscribe to request subject")
	}

	if s.replySubscription, err = s.conn.subscribe(s.replySubjectPrefix+".response", false, s.handleReply); err != nil {
		return errors.Wrapf(err, "failed to subscribe to reply subject")
	}

	//creat a client that can be used from contexts
	clientConfig := ClientConfig{
		Config: Config{
			Domain: s.config.Domain,
		},
	}
	log.Debugf("client config: %+v", clientConfig)
	if err := clientConfig.Validate(); err != nil {
		panic(fmt.Sprintf("client config: %+v", err))
	}
	s.client, err = clientConfig.Create()
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %+v", err))
	}

	//todo: graceful shutdown
	log.Debugf("NATS service(%s) running...", s.config.Domain)
	x := make(chan bool)
	<-x
	log.Debugf("NATS service(%s) stopped...", s.config.Domain)
	return nil
} //server.Serve()

//context value identifiers
type CtxClient struct{}

func (s *server) handleRequest(msg received) {
	log.Debugf("Received %s", string(msg.data))

	var request Request
	var err error
	var responseData interface{}
	defer func() {
		if err != nil {
			//response is error
			log.Errorf("Error response: %+v", err)
		} else {
			log.Debugf("Success response type: %T", responseData)
		}

		//reply only if has reply topic, determined as follows:
		// - sync request where client waits for a response, specifies NATS msg.replySubject
		//		and should only be used when sending client process must get the response
		//		and not when any other instance of the same process can process the response
		//		(the latter is preferred for resilience)
		// - async requests does not keep the thread open and the reply is sent
		//		to NATS topic "<request.client.domain>.response"
		//		this can still be sent to the same process if the domain includes a unique
		//		instance id.
		// - the response can be sent on to any other service by setting request.forward
		//		(when set, request.client will not be used for sending the response)
		// - to fire-and-forget, set request.no_response=true and do not send NATS msg.replySubject
		if request.NoResponse {
			log.Debugf("response silenced")
		} else {
			replySubject := msg.replySubject //when set, overrides any other logic from the request
			if replySubject == "" {
				if request.Forward != nil {
					replySubject = request.Forward.Domain + ".response"
				} else {
					if request.Client != nil {
						replySubject = request.Client.Domain + ".response"
					}
				}
			}

			if replySubject == "" {
				log.Debugf("no reply subject - not sending a reply")
			} else {
				response := Response{
					Header: request.Header,
				}

				if err != nil {
					response.Errors = []ms.Error{
						{
							Code:    "failed", //todo fmt.Sprintf("%v", err.Code()), //todo: get code & code stack from err
							Details: fmt.Sprintf("%+s", err),
							Source:  fmt.Sprintf("%+v", err),
						},
					}
					//todo: support for full error stack and append to called service errors...
				} else {
					response.Data = responseData
				}

				log.Debugf("reply to %s: %+v", replySubject, response)
				jsonRes, _ := json.Marshal(response)
				log.Debugf("reply to %s: %s", replySubject, string(jsonRes))
				if err := s.conn.send(nil, replySubject, jsonRes); err != nil {
					log.Errorf("failed to reply to %s: %+v", replySubject, err)
				} else {
					log.Debugf("replied successfully to %s: %s", replySubject, string(jsonRes))
				}
			} //if has reply subject
		} //if response not silenced
	}()

	if err = json.Unmarshal(msg.data, &request); err != nil {
		err = errors.Wrapf(err, "cannot unmarshal JSON into %T", string(msg.data), request)
		return
	}

	log.Debugf("RECV: %+v", request)
	if err = request.Validate(); err != nil {
		err = errors.Wrapf(err, "invalid request message")
		return
	}

	//determine operation name from provider.name="/<domain>/<operName>"
	o, ok := s.svc.Oper(request.Service.Operation)
	if !ok {
		err = errors.Errorf("unknown operName(%+v)", request.Service)
		return
	}

	var reqValuePtr reflect.Value
	if o.ReqType() != nil {
		if request.Data == nil {
			err = errors.Errorf("missing request data when expecting %v", o.ReqType())
			return
		}
		reqValuePtr = reflect.New(o.ReqType())
		var jsonRequest []byte
		jsonRequest, err = json.Marshal(request.Data)
		if err != nil {
			err = errors.Wrapf(err, "failed to marshal request to JSON so that it can be unmarshalled into %v", o.ReqType())
		}
		if err = json.Unmarshal(jsonRequest, reqValuePtr.Interface()); err != nil {
			err = errors.Wrapf(err, "failed to decode request into %v", o.ReqType())
			return
		}
		if validator, ok := (reqValuePtr.Interface()).(ms.Validator); ok {
			if err = validator.Validate(); err != nil {
				err = errors.Wrapf(err, "invalid %v", o.ReqType())
				return
			}
		}
		log.Debugf("oper(%s) request: (%T)%+v", request.Service.Operation, reqValuePtr.Elem().Interface(), reqValuePtr.Elem().Interface())
	} else {
		log.Debugf("oper(%s) no request", request.Service.Operation)
	}

	//has a valid request
	ctx := context.Background()
	ctx = context.WithValue(ctx, ms.CtxID{}, request.ContextID) //may be same as other context, but request ID will/must be unique
	ctx = context.WithValue(ctx, CtxClient{}, s.client)         //todo: client wrapper with context id and ability to determine remaining time?

	//todo: ctx logger - with sync and discard/log and level controls to filter how much is written

	//todo: auditing

	//todo: consider adding .1, .2, ... to each outgoing context ID or request ID when making client calls, so can still match but unique
	//think well over purpose of having both or just one combined field...

	//call the operation handler function
	responseData, err = o.Handle(ctx, reqValuePtr.Elem().Interface())
	if err != nil {
		log.Debugf("Handler -> error: %+v", err)
		err = errors.Wrapf(err, "oper(%s) handler failed", o.Name())
	}
	log.Debugf("Handler -> res: (%T)%+v", responseData, responseData)
} //handler.HandleRequest()

// handleReply() handles reply messages from nats after we sent with conn.Request()
func (s *server) handleReply(msg received) {
	log.Debugf("Received reply \"%s\" on subject %s", msg.data, msg.subject)
	var replyChan chan received
	var ok bool
	key := msg.subject
	s.replyChannelsLock.Lock()
	if replyChan, ok = s.replyChannels[key]; !ok {
		s.replyChannelsLock.Unlock()
		log.Errorf("%+v", errors.Errorf("reply key(%s) not found, discarding \"%s\"", key, msg.data))
		return
	}
	delete(s.replyChannels, key)
	s.replyChannelsLock.Unlock()
	replyChan <- msg
	close(replyChan)
	//log.Tracef("Replied for %s", key)
} //handler.handleReply()

// // SendReply sends a reply to the reply queue of domain and operation
// func (handler *NatsHandler) SendReply(message *Message) error {

// 	return handler.SendSubject(
// 		message.Header.ReplyAddress,
// 		message)

// } // NatsHandler.SendReply()

// // Send sends message to Nats
// func (handler *NatsHandler) Send(message *Message) error {

// 	return handler.SendSubject(
// 		"",
// 		message)

// } // NatsHandler.Send()

// // SendAndReceive sends a message on Nats, and waits for a reply
// func (handler *NatsHandler) SendAndReceive(reqMessage *Message, resMessage *Message) error {

// 	const method = "NatsHandler.SendAndReceive"

// 	if handler == nil {
// 		return errors.Errorf("invalid parameters %p.%s ()",
// 			handler,
// 			method)
// 	} // if invalid params

// 	if err := handler.SendAndReceiveSubject(
// 		"",
// 		reqMessage,
// 		resMessage); err != nil {

// 		return errors.Wrapf(err, "Error sending message")

// 	} // if failed to send

// 	return nil

// } // NatsHandler.SendAndReceive()

// // SendAndReceiveSubject sends a message on Nats on a given subject, and waits
// // for a reply
// func (handler *NatsHandler) SendAndReceiveSubject(subject string, reqMessage *Message,
// 	resMessage *Message) error {

// 	const method = "NatsHandler.SendAndReceiveSubject"

// 	if handler == nil || reqMessage == nil || resMessage == nil {
// 		return errors.Errorf("invalid parameters %p.%s ()",
// 			handler,
// 			method)
// 	} // if invalid params

// 	log := natsLogger.Named(method)
// 	defer log.Sync()

// 	if len(subject) <= 0 {
// 		subject = strings.Replace(reqMessage.Header.Provider.Name, "/", ".", -1)
// 		subject = strings.TrimSpace(strings.Replace(subject, ".", "", 1))
// 	} // if subject not supplied

// 	// Get the reply subject and set it in the header
// 	replySubject := handler.newReplySubject()
// 	reqMessage.Header.ReplyAddress = replySubject

// 	// Convert the message to JSON
// 	msgData, err := reqMessage.ToJSON()
// 	if err != nil {
// 		*resMessage = *reqMessage
// 		return errors.Wrap(err,
// 			"Failed to convert message to JSON")
// 	} // failed to get json msg

// 	log.Debugf("Sending Message \"%s\" on subject %s. "+
// 		"Expecting reply on subject %s.",
// 		msgData,
// 		subject,
// 		replySubject)

// 	// Make a buffered channel to prevent the go routine potentially
// 	// blocking when sending data into the channel. This can happen when
// 	// attempting to send after the receiving go routine has timedout.
// 	replyChan := make(chan *nats.Msg, 1)

// 	handler.replyChannelsLock.Lock()
// 	if _, ok := handler.replyChannels[replySubject]; ok {
// 		handler.replyChannelsLock.Unlock()
// 		return errors.Errorf("Reply subject %s already added",
// 			replySubject)
// 	} // if key
// 	handler.replyChannels[replySubject] = replyChan
// 	handler.replyChannelsLock.Unlock()

// 	// Defer remove the key from the map
// 	defer func() {

// 		log.Tracef("Attempting to remove reply channel for %s",
// 			replySubject)

// 		handler.replyChannelsLock.Lock()
// 		delete(handler.replyChannels, replySubject)
// 		handler.replyChannelsLock.Unlock()

// 		log.Tracef("Reply channel removed for %s",
// 			replySubject)

// 	}() // defer ()

// 	// Send the message
// 	sendMsg := nats.NewMsg(subject)
// 	sendMsg.Reply = replySubject
// 	sendMsg.Data = []byte(msgData)
// 	if handler.headersSupported {
// 		sendMsg.Header.Set(headerVServicesProvider, reqMessage.Header.Provider.Name)
// 	} // if headers

// 	if err = handler.conn.PublishMsg(
// 		sendMsg); err != nil {

// 		*resMessage = *reqMessage
// 		return errors.Wrapf(err, "Failed to publish request on subject %s",
// 			subject)

// 	} // if failed to publish

// 	// Wait for response
// 	ttl := time.Duration(reqMessage.Header.Ttl) * time.Millisecond
// 	log.Debugf("Waiting for reply on %s with TTL %s",
// 		replySubject,
// 		ttl)

// 	select {
// 	case replyMsg := <-replyChan:

// 		if len(replyMsg.Data) <= 0 {
// 			err := errors.Errorf("No responders for subject %s for message with GUID %s",
// 				subject,
// 				reqMessage.Header.IntGuid)
// 			log.Errorf("%+v", err)
// 			*resMessage = *reqMessage
// 			reqMessage.Header.Result = &Result{Code: -99, Description: "No responders", Details: err.Error()}
// 			return nil
// 		} // if no data

// 		if err := resMessage.FromJSON(
// 			string(replyMsg.Data)); err != nil {

// 			*resMessage = *reqMessage
// 			return errors.Wrapf(err,
// 				"Failed to create message from JSON [%s]",
// 				replyMsg.Data)

// 		} // if failed to create from JSON

// 		resMessage.Header.ReplyAddress = strings.TrimPrefix(
// 			resMessage.Header.ReplyAddress,
// 			"Q:")

// 		if !strings.Contains(resMessage.Header.ReplyAddress, ".") {
// 			resMessage.Header.ReplyAddress = resMessage.Header.ReplyAddress + ".reply"
// 		} // if does not include operation

// 		return nil

// 	case <-time.After(ttl):
// 		err := errors.Errorf("Timeout after %s waiting for reply on subject %s from provider %s for GUID %s",
// 			ttl,
// 			replySubject,
// 			reqMessage.Header.Provider.Name,
// 			reqMessage.Header.IntGuid)
// 		log.Errorf("%+v", err)
// 		*resMessage = *reqMessage
// 		reqMessage.Header.Result = &Result{Code: -99, Description: "Request Timed out", Details: err.Error()}
// 		return nil
// 	} // select

// } // NatsHandler.SendAndReceiveSubject()

// // SubscribeWithNoFilter ...
// func (handler *NatsHandler) SubscribeWithNoFilter(subject string,
// 	callback HandlerSubscribe) error {

// 	const method = "NatsHandler.SubscribeWithNoFilter"

// 	if handler == nil {
// 		return errors.Errorf("invalid parameters %p.%s ()",
// 			handler,
// 			method)
// 	} // if invalid params

// 	log := natsLogger.Named(method)
// 	defer log.Sync()

// 	handler.subscriptionsLock.Lock()
// 	defer handler.subscriptionsLock.Unlock()

// 	if _, ok := handler.subscriptions[subject]; ok {

// 		log.Errorf("%+v", errors.Errorf("Subject [%s] already subscribed to",
// 			subject))
// 		return nil

// 	} // if exists

// 	var subscription *nats.Subscription
// 	var err error

// 	if !handler.config.NormalSubscription {

// 		subscription, err = handler.conn.QueueSubscribe(subject, fmt.Sprintf("Q.%s", subject), func(msg *nats.Msg) {
// 			callback(msg.Data, msg.Reply)
// 		})

// 		if err != nil {
// 			log.Errorf("Queue Subscribe failed. Error: [%s]",
// 				err.Error())
// 		} // if failed

// 	} else {

// 		subscription, err = handler.conn.Subscribe(subject+".*", func(msg *nats.Msg) {
// 			callback(msg.Data, msg.Reply)
// 		})

// 		if err != nil {
// 			log.Errorf("Subscribe failed. Error: [%s]",
// 				err.Error())
// 		} // if failed

// 	}

// 	handler.subscriptions[subject] = subscription
// 	handler.defaultReplyQ = subject + ".reply"
// 	return nil

// } //NatsHandler.SubscribeWithNoFilter()

// // UnSubscribe unsubscribes from the given subject
// func (handler *NatsHandler) UnSubscribe(subject string) error {

// 	const method = "NatsHandler.UnSubscribe"

// 	if handler == nil {
// 		return errors.Errorf("invalid parameters %p.%s ()",
// 			handler,
// 			method)
// 	} // if invalid params

// 	handler.subscriptionsLock.Lock()
// 	defer handler.subscriptionsLock.Unlock()

// 	if subscription, ok := handler.subscriptions[subject]; ok {

// 		if err := subscription.Unsubscribe(); err != nil {
// 			return errors.Wrapf(err,
// 				"Failed to unsubscribe")
// 		} // if failed to un-subscribe

// 		delete(handler.subscriptions, subject)

// 	} // if exists

// 	return nil

// } // RedisHandler.UnSubscribe()

// func (handler *NatsHandler) Terminate() error {

// 	const method = "NatsHandler.Terminate"

// 	if handler == nil {
// 		return errors.Errorf("invalid parameters %p.%s ()",
// 			handler,
// 			method)
// 	} // if invalid params

// 	handler.subscriptionsLock.Lock()
// 	defer handler.subscriptionsLock.Unlock()

// 	for _, subscription := range handler.subscriptions {
// 		if subscription != nil {
// 			if err := subscription.Unsubscribe(); err != nil {
// 				return errors.Wrapf(err,
// 					"error un-subscribing")
// 			} // if failed to un sub
// 		} // if subscription
// 	} // for each subscription

// 	handler.subscriptions = make(map[string]*nats.Subscription)

// 	return nil

// } // NatsHandler.Terminate()

// // Generate a new reply subject
// func (handler *NatsHandler) newReplySubject() string {
// 	// Max length that NGF can handle is 63
// 	var sb strings.Builder
// 	n := nuid.Next()
// 	sb.Grow(len(handler.replySubjectPrefix) + len(n))
// 	sb.WriteString(handler.replySubjectPrefix)
// 	sb.WriteString(n)
// 	return sb.String()
// } // NatsHandler.newReplySubject()

// // DefaultReplyQ ...
// func (handler *NatsHandler) DefaultReplyQ() string {
// 	return handler.defaultReplyQ
// } // NatsHandler.DefaultReplyQ()
