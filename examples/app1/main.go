package main

import (
	"context"
	"strings"
	"time"

	"github.com/go-msvc/config"
	"github.com/go-msvc/errors"
	"github.com/go-msvc/logger"
	"github.com/go-msvc/nats-utils"
	_ "github.com/go-msvc/nats-utils"
	"github.com/go-msvc/nats-utils/examples/app2/two"
	"github.com/go-msvc/utils/ms"
)

var log = logger.New().WithLevel(logger.LevelDebug)

func main() {
	svc := ms.New(
		ms.WithOper("oper1", oper1),
		ms.WithOper("oper2", oper2))
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

//=====[ oper2 ]========================================
// this operation calls app2.oper2 and then mangle the
// response so you can see it came from app1
//======================================================
type Oper2Request struct {
	Name string `json:"name"`
}

type Oper2Response struct {
	Message string `json:"message"`
}

func oper2(ctx context.Context, req Oper2Request) (*Oper2Response, error) {
	//get client
	//todo: this must come from ms using the service registry to determine the type of client
	//required for the called service, and cache that info
	//but for now - all are NATS clients and the server provides it in ctx
	cli, ok := ctx.Value(nats.CtxClient{}).(ms.Client)
	if !ok {
		return nil, errors.Errorf("no client to use")
	}
	timeRemain := time.Second //todo - get from ctx - let client get it!

	//call app2.oper2
	app2ResValue, err := cli.Sync(ctx,
		ms.Address{
			Domain:    "app2",
			Operation: "oper2",
		},
		timeRemain,
		two.Oper2Request{
			Name: strings.ToUpper(req.Name),
		},
		two.Oper2Response{})
	log.Debugf("app2Res: (%T)%+v", app2ResValue, app2ResValue)
	if err != nil {
		log.Debugf("failed to call: %+v", err)
		return nil, errors.Wrapf(err, "failed to call app2")
	}
	log.Debugf("app2Res: (%T)%+v", app2ResValue, app2ResValue)
	app2Res, ok := app2ResValue.(two.Oper2Response)
	if !ok {
		return nil, errors.Wrapf(err, "not expected %T", app2ResValue)
	}
	log.Debugf("app2Res: (%T)%+v", app2Res, app2Res)

	return &Oper2Response{
		Message: "Got from app2(" + app2Res.Message + ")",
	}, nil
}
