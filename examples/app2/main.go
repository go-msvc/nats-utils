package main

import (
	"context"
	"time"

	"github.com/go-msvc/config"
	"github.com/go-msvc/errors"
	_ "github.com/go-msvc/nats-utils"
	"github.com/go-msvc/utils/ms"
)

func main() {
	svc := ms.New(
		ms.WithOper("oper2", oper2))
	if err := config.AddSource("file", config.File("./config.json")); err != nil {
		panic(err)
	}
	if err := config.Load(); err != nil {
		panic(err)
	}
	svc.Serve()
}

type Oper2Request struct {
	Name string `json:"name"`
}

type Oper2Response struct {
	Message string `json:"message"`
}

func oper2(ctx context.Context, req Oper2Request) (*Oper2Response, error) {
	if time.Now().Second()%5 < 1 {
		return nil, errors.Errorf("oper2 sometimes fails")
	}
	return &Oper2Response{
		Message: "Goodbye " + req.Name,
	}, nil
}
