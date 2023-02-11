package nats

import (
	"time"

	"github.com/go-msvc/errors"
	"github.com/go-msvc/nats-utils/datatype"
	"github.com/go-msvc/utils/ms"
)

type Request struct {
	Header
	Forward    *ms.Address       `json:"forward,omitempty" doc:"Specify to send response to other place than the client."`
	TTL        datatype.Duration `json:"ttl,omitempty" doc:"Discard request when received after timestamp+ttl and do not reply later than this."`
	Data       interface{}       `json:"data,omitempty" doc:"Request data"`
	NoResponse bool              `json:"no_response,omitempty" doc:"Set true to silence response success/error"`
}

func (r Request) Validate() error {
	if err := r.Header.Validate(); err != nil {
		return errors.Wrapf(err, "invalid header")
	}
	if r.Forward != nil {
		if err := r.Forward.Validate(); err != nil {
			return errors.Wrapf(err, "invalid forward address")
		}
	}
	if r.TTL <= 0 {
		return errors.Errorf("ttl not positive")
	}
	return nil
} //Request.Validate()

type Response struct {
	Header
	Errors []ms.Error  `json:"errors,omitempty" doc:"Description of error stack when failed"`
	Data   interface{} `json:"data,omitempty" doc:"Response data when succeeded and operation has response data."`
}

type Header struct {
	Timestamp time.Time   `json:"timestamp"` //todo: datatype.Timestamp
	ContextID string      `json:"context_id,omitempty" doc:"Context ID should be unique per context to group related messages, and is echoed in a response."`
	RequestID string      `json:"request_id,omitempty" doc:"Request ID should be unique for every request, even in the same context, and is echoed in a response."`
	Client    *ms.Address `json:"client,omitempty" doc:"Request sent from this client"`
	Service   *ms.Address `json:"service,omitempty" doc:"Request sent to this service"`
}

func (h Header) Validate() error {
	if h.Timestamp.IsZero() {
		return errors.Errorf("missing timestamp")
	}
	if h.ContextID == "" {
		return errors.Errorf("missing context_id")
	}
	if h.RequestID == "" {
		return errors.Errorf("missing request_id")
	}
	if h.Client == nil {
		return errors.Errorf("missing client address")
	}
	if h.Service == nil {
		return errors.Errorf("missing service address")
	}
	if err := h.Client.Validate(); err != nil {
		return errors.Wrapf(err, "invalid client address")
	}
	if err := h.Service.Validate(); err != nil {
		return errors.Wrapf(err, "invalid service address")
	}
	return nil
} //Header.Validate()

//todo:
//	- authentication info in request?
//	- move this to utils/ms?
