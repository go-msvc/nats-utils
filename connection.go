package nats

import (
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/go-msvc/errors"
	"github.com/nats-io/nats.go"
)

var (
	connectionMutex sync.Mutex
	theConnection   *connection
)

// connection to nats server, used by both server and client so each process has only one of these
type connection struct {
	config            Config
	conn              *nats.Conn
	headersSupported  bool
	subscriptionsLock sync.Mutex
	subscriptions     map[string]*nats.Subscription
}

type received struct {
	subject      string
	data         []byte
	replySubject string
}

type handlerFunc func(received)

func connect(c Config) (*connection, error) {
	connectionMutex.Lock()
	defer connectionMutex.Unlock()
	if theConnection == nil {
		newConnection := &connection{
			config: c,
		}

		var options []nats.Option
		options = append(options, nats.Name(c.Domain))
		options = append(options, nats.Timeout(c.Timeout.Duration()))
		options = append(options, nats.MaxReconnects(c.MaxReconnects))
		options = append(options, nats.ReconnectWait(c.ReconnectWait.Duration()))
		options = append(options, nats.ReconnectJitter(c.ReconnectJitter.Duration(), c.ReconnectJitterTls.Duration()))
		options = append(options, nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Errorf("Reconnecting %+v\n", conn)
		}))
		if c.DontRandomize {
			options = append(options, nats.DontRandomize())
		}
		options = append(options, nats.UserInfo(c.Username, c.Password.StringPlain()))
		options = append(options, nats.Token(c.Token))
		if c.Secure {
			options = append(options, nats.Secure(&tls.Config{InsecureSkipVerify: c.InsecureSkipVerify}))
		}

		newConnection.subscriptions = make(map[string]*nats.Subscription)

		var err error
		newConnection.conn, err = nats.Connect(c.Url, options...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to NATS")
		}
		newConnection.headersSupported = newConnection.conn.HeadersSupported()
		theConnection = newConnection
	}
	return theConnection, nil
} //connect()

// Subscribe() to group queue (only one instance get the request) or broadcast
// queue (each instance get it)
func (s *connection) subscribe(subject string, broadcast bool, hdlr handlerFunc) (*nats.Subscription, error) {
	if s == nil {
		return nil, errors.Errorf("nil.Subscribe()")
	}
	s.subscriptionsLock.Lock()
	defer s.subscriptionsLock.Unlock()
	if ss, ok := s.subscriptions[subject]; ok {
		return ss, nil //already subscribed, assuming with same callback
	}
	var subscription *nats.Subscription
	var err error
	if !broadcast {
		subscription, err = s.conn.QueueSubscribe(
			subject+".*",
			fmt.Sprintf("Q.%s", subject),
			func(msg *nats.Msg) {
				hdlr(received{subject: msg.Subject, data: msg.Data, replySubject: msg.Reply})
			},
		)
		if err != nil {
			return nil, errors.Wrapf(err, "queue subscribe(%s) failed", subject)
		}
	} else {
		subscription, err = s.conn.Subscribe(
			subject+".*",
			func(msg *nats.Msg) {
				hdlr(received{subject: msg.Subject, data: msg.Data, replySubject: msg.Reply})
			},
		)
		if err != nil {
			return nil, errors.Wrapf(err, "subscribe(%s) failed", subject)
		}
	}
	s.subscriptions[subject] = subscription
	//h.defaultReplyQ = subject + ".reply"
	return subscription, nil
} //connection.Subscribe()

// Send() sends a message to Nats on a given subject
func (c *connection) Send(header map[string]string, subject string, data []byte) error {
	if c == nil {
		return errors.Errorf("nil.Send()")
	}
	sendMsg := nats.NewMsg(subject)
	sendMsg.Data = []byte(data)
	if c.headersSupported {
		for n, v := range header {
			sendMsg.Header.Set(n, v)
		}
	}
	if err := c.conn.PublishMsg(sendMsg); err != nil {
		return errors.Wrap(err, "failed to publish message")
	}
	return nil
} //handler.Send()
