package main

import (
	"github.com/31333337/bmrng/go/0kn/pkg/logger"
	"github.com/31333337/bmrng/go/trellis/client"
	"github.com/31333337/bmrng/go/trellis/config"
	"github.com/31333337/bmrng/go/trellis/errors"
	"github.com/31333337/bmrng/go/trellis/network"
)

// from trellis/cmd/client/
func LaunchClient(args ArgsClient) {
	defer logger.Sugar.Sync()

	serversFile := args.ServerFile
	groupsFile := args.GroupFile
	clientsFile := args.ClientFile
	addr := args.Addr
	errors.Addr = addr

	logger.Sugar.Infow(
		"Launching client",
		"address", addr,
	)

	servers, err := config.UnmarshalServersFromFile(serversFile)
	if err != nil {
		logger.Sugar.Fatalw("Could not read servers file",
			"file", serversFile,
			"error", err,
		)
	}

	groups, err := config.UnmarshalGroupsFromFile(groupsFile)
	if err != nil {
		logger.Sugar.Fatalw("Could not read group file",
			"file", groupsFile,
			"error", err,
		)
	}

	clients, err := config.UnmarshalServersFromFile(clientsFile)
	if err != nil {
		logger.Sugar.Fatalw("Could not read clients file",
			"file", clientsFile,
			"error", err,
		)
	}

	clientRunner := client.NewClientRunner(servers, groups)

	// This fails @ network/rpc_call.go callee.HandleSignedMessageStream
	/*
		err = clientRunner.Connect()
		if err != nil {
			log.Fatalf("Could not make clients %v", err)
		}
	*/

	network.RunServer(nil, clientRunner, clients, addr)
}
