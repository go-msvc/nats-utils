package two

import (
	_ "github.com/go-msvc/nats-utils"
)

type Oper2Request struct {
	Name string `json:"name"`
}

type Oper2Response struct {
	Message string `json:"message"`
}
