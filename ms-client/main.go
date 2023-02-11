package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-msvc/errors"
	"github.com/go-msvc/logger"
	"github.com/go-msvc/nats-utils"
	"github.com/go-msvc/utils/ms"
)

var log = logger.New().WithLevel(logger.LevelDebug)

func main() {
	clientDomain := flag.String("client", "ms-client", "Client domain name")
	serviceDomain := flag.String("D", "", "Service domain name")
	serviceOper := flag.String("O", "", "Service operation name")
	serviceTTL := flag.Int("ttl", 2000, "Service timeout (milliseconds: must be >= 100)")
	serviceReqJSON := flag.String("R", "{}", "Service request data on command line")
	serviceReqFile := flag.String("F", "", "Service request file to load")
	flag.Parse()

	if len(*serviceDomain) == 0 {
		panic("option for service domain -D ... is required")
	}
	if len(*serviceOper) == 0 {
		panic("option for service operarion -O ... is required")
	}
	if *serviceTTL < 100 {
		panic(errors.Errorf("service ttl %d should be >= 1000 (ms)", serviceTTL))
	}
	if len(*clientDomain) == 0 {
		panic("client domain is required")
	}

	var reqData interface{}
	if len(*serviceReqFile) > 0 {
		if *serviceReqJSON != "{}" {
			panic(errors.Errorf("do not use both -R and -F options"))
		}
		f, err := os.Open(*serviceReqFile)
		if err != nil {
			panic(errors.Wrapf(err, "cannot open request file %s", serviceReqFile))
		}
		if err := json.NewDecoder(f).Decode(&reqData); err != nil {
			panic(errors.Wrapf(err, "invalid JSON in request file %s", serviceReqFile))
		}
		f.Close()
	} else {
		if err := json.Unmarshal([]byte(*serviceReqJSON), &reqData); err != nil {
			panic(errors.Wrapf(err, "invalid JSON in -R ... option"))
		}
	}

	clientConfig := nats.ClientConfig{
		Config: nats.Config{
			Domain: "ms-client", //todo: use uuid?
		},
	}
	if err := clientConfig.Validate(); err != nil {
		panic(fmt.Sprintf("client config: %+v", err))
	}

	client, err := clientConfig.Create()
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %+v", err))
	}

	//ms-client use one id for context, request and own domain, as it does only one request then terminates
	ctx := context.Background()
	res, err := client.Sync(
		ctx,
		ms.Address{
			Domain:    *serviceDomain,
			Operation: *serviceOper,
		},
		time.Millisecond*time.Duration(*serviceTTL),
		reqData,
		nil)
	if err != nil {
		panic(fmt.Sprintf("failed to do request: %+v", err))
	}
	log.Debugf("Got res (%T)%+v", res, res)
} //main()
