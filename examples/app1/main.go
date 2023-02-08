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
	svc := ms.New("app1",
		ms.WithOper("oper1", oper1))
	if err := config.AddSource("file", config.File("./config.json")); err != nil {
		panic(err)
	}
	if err := config.Load(); err != nil {
		panic(err)
	}
	svc.Serve()
}

type Oper1Request struct {
	Name string `json:"name"`
}

type Oper1Response struct {
	Message string `json:"message"`
}

func oper1(ctx context.Context, req Oper1Request) (*Oper1Response, error) {
	if time.Now().Second()%5 < 1 {
		return nil, errors.Errorf("oper1 sometimes fails")
	}
	return &Oper1Response{
		Message: "Hello " + req.Name,
	}, nil
}
