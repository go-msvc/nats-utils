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
	// "bitbucket.org/vservices/utils/v4/env"
	// "bitbucket.org/vservices/utils/v4/errors"
	// "bitbucket.org/vservices/utils/v4/integration"
	// "bitbucket.org/vservices/utils/v4/logger"
	// "bitbucket.org/vservices/utils/v4/ms"
	// "github.com/spf13/viper"
)

var log = logger.New().WithLevel(logger.LevelDebug)

func main() {
	clientDomain := flag.String("client", "ms-client", "Client domain name")
	serviceDomain := flag.String("D", "", "Service domain name")
	serviceOper := flag.String("O", "", "Service operation name")
	serviceTTL := flag.Int("ttl", 15000, "Service timeout (milliseconds)")
	serviceReqJSON := flag.String("R", "{}", "Service request data on command line")
	serviceReqFile := flag.String("F", "", "Service request file to load")
	//msProt := flag.String("prot", "nats", "Messaging protocol (nats|redis)")
	flag.Parse()

	if len(*serviceDomain) == 0 {
		panic("option for service domain -D ... is required")
	}
	if len(*serviceOper) == 0 {
		panic("option for service operarion -O ... is required")
	}
	if *serviceTTL < 1000 {
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

	log.Debugf("1")
	clientConfig := nats.ClientConfig{
		Config: nats.Config{
			Domain: "ms-client", //todo: use uuid?
		},
	}
	log.Debugf("1")
	if err := clientConfig.Validate(); err != nil {
		panic(fmt.Sprintf("client config: %+v", err))
	}

	log.Debugf("1")
	client, err := clientConfig.Create()
	log.Debugf("1")
	if err != nil {
		log.Debugf("1")
		panic(fmt.Sprintf("failed to create client: %+v", err))
	}

	//ms-client use one id for context, request and own domain, as it does only one request then terminates
	ctx := context.Background()
	log.Debugf("1: c=(%T)", client)
	res, err := client.Sync(
		ctx,
		ms.Address{
			Domain:    *serviceDomain,
			Operation: *serviceOper,
		},
		time.Minute, reqData)
	log.Debugf("1: c=(%T)", client)
	if err != nil {
		log.Debugf("1")
		panic(fmt.Sprintf("failed to do request: %+v", err))
	}

	log.Debugf("1")
	log.Debugf("Got res (%T)%+v", res, res)
} //main()
