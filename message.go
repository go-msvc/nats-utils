package nats

import (
	"time"

	"github.com/go-msvc/nats-utils/datatype"
	"github.com/go-msvc/utils/ms"
)

type Request struct {
	Header
	TTL     datatype.Duration `json:"ttl,omitempty" doc:"Discard request when received after timestamp+ttl and do not reply later than this."`
	Service ms.Address        `json:"service" doc:"Sent to this service for processing"`
	Data    interface{}       `json:"data,omitempty" doc:"Request data"`
	ReplyTo ms.Address        `json:"reply_to,omitempty" doc:"Path to reply to"`
}

type Response struct {
	Header
	Errors []ms.Error  `json:"errors,omitempty" doc:"Description of error stack when failed"`
	Data   interface{} `json:"data,omitempty" doc:"Response data when succeeded and operation has response data."`
}

type Header struct {
	Timestamp time.Time `json:"timestamp"` //todo: datatype.Timestamp
	ContextID string    `json:"context_id" doc:"Context ID"`
	RequestID string    `json:"request_id" doc:"Request ID"`
}
