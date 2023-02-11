package main

import (
	"context"
	"time"

	"github.com/go-msvc/config"
	"github.com/go-msvc/errors"
	_ "github.com/go-msvc/nats-utils"
	"github.com/go-msvc/nats-utils/examples/app2/two"
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

// req/res structs are in sub dir ./two so they can be
// imported by the client
func oper2(ctx context.Context, req two.Oper2Request) (*two.Oper2Response, error) {
	if time.Now().Second()%5 < 1 {
		return nil, errors.Errorf("oper2 sometimes fails")
	}
	return &two.Oper2Response{
		Message: "Goodbye " + req.Name,
	}, nil
}
