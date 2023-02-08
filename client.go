package nats

import (
	"github.com/go-msvc/errors"
	"github.com/go-msvc/utils/ms"
)

type client struct {
	config ClientConfig
}

func (c client) Sync(addr ms.Address, req interface{}) (res interface{}, err error) {
	return nil, errors.Errorf("NYI")
}
func (c client) ASync(addr ms.Address, req interface{}) (err error) {
	return errors.Errorf("NYI")
}
func (c client) Send(addr ms.Address, req interface{}) (err error) {
	return errors.Errorf("NYI")
}
