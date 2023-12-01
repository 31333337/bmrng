package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	arg "github.com/alexflint/go-arg"

	"github.com/31333337/bmrng/go/0kn/pkg/gateway"
	"github.com/31333337/bmrng/go/0kn/pkg/logger"
	"github.com/31333337/bmrng/go/0kn/pkg/utils"
	"github.com/31333337/bmrng/go/trellis/config"
	"github.com/31333337/bmrng/go/trellis/coordinator"
)

const (
	// run network in the same process
	NETWORK_TYPE_IN_PROCESS = iota

	// run local network in separate processes on the same machine
	NETWORK_TYPE_LOCAL

	// run network on remote hosts
	NETWORK_TYPE_REMOTE
)

func LaunchCoordinator(args ArgsCoordinator, argParser *arg.Parser) {
	processArgs(&args, argParser)

	switch {
	case args.Config != nil:
		runConfigGenerator(args)

	case args.Experiment != nil:
		runExperiment(args)

	case args.Mixnet != nil:
		runMixnet(args)
	}
}

func processArgs(args *ArgsCoordinator, argParser *arg.Parser) {
	defer logger.Sugar.Sync()

	if args.GroupSize == 0 {
		if args.F != 0 {
			if args.NumGroups != 0 {
				args.GroupSize = config.CalcGroupSize(args.NumServers, args.NumGroups, args.F)
			} else {
				args.GroupSize, args.NumGroups = config.CalcFewGroups2(args.F, args.NumServers)
			}
		} else {
			argParser.WriteHelp(os.Stdout)
			logger.Sugar.Fatal("Error: set groupsize or f")
			return
		}
	}

	if args.NumGroups == 0 {
		// Integer division takes floor
		args.NumGroups = args.NumServers / args.GroupSize
	}

	if args.NumLayers == 0 {
		if args.F != 0 {
			args.NumLayers = config.NumLayers(args.NumUsers, args.F)
		} else {
			argParser.WriteHelp(os.Stdout)
			logger.Sugar.Fatal("Error: set numlayers or f")
			return
		}
	}

	if args.BinSize == 0 {
		args.BinSize = config.BinSize2(
			args.NumLayers,
			args.NumServers,
			args.NumUsers,
			-args.Overflow,
		)
	}

	if args.LimitSize == 0 {
		// the limit for how many servers needs to check the boomerang message
		// the size of an anytrust group ensures one honest server
		// we need replacement because servers can be selected multiple times
		args.LimitSize = config.GroupSizeWithReplacement(args.NumGroups, args.F)
	}

	if args.LoadMessages {
		args.NumClientServers = 0
	}

	logger.Sugar.Infow(
		"xtrellis args",
		"args", args,
	)
}

func setupNetwork(args ArgsCoordinator) *coordinator.CoordinatorNetwork {
	defer logger.Sugar.Sync()

	var net *coordinator.CoordinatorNetwork

	switch args.NetworkType {
	case NETWORK_TYPE_IN_PROCESS: // run in the same process
		net = coordinator.NewInProcessNetwork(args.NumServers, args.NumGroups, args.GroupSize)

	case NETWORK_TYPE_LOCAL: // run in separate process on the same machine
		serverConfigs, groupConfigs, clientConfigs := coordinator.NewLocalConfig(
			args.NumServers,
			args.NumGroups,
			args.GroupSize,
			args.NumClientServers,
			false,
		)

		// copy public server config to private servers file
		// TODO: improve public/private config generation
		serverPrivateFile := args.ServerPrivateFile
		if err := config.MarshalServersToFile(serverPrivateFile, serverConfigs); err != nil {
			logger.Sugar.Fatalw("Could not write private servers file",
				"file", serverPrivateFile,
				"error", err,
			)
		}

		if args.LoadMessages {
			oldServers, err := config.UnmarshalServersFromFile(args.ServerFile)
			if err != nil {
				logger.Sugar.Fatalw("Could not read servers file",
					"file", args.ServerFile,
					"error", err,
				)
			}
			// copy old keys
			for id, s := range serverConfigs {
				old := oldServers[id]
				s.PrivateKey = old.PrivateKey
				s.PublicKey = old.PublicKey
			}
		}
		net = coordinator.NewLocalNetwork(serverConfigs, groupConfigs, clientConfigs)

	case NETWORK_TYPE_REMOTE: // run on remote hosts
		clientFile := args.ClientFile
		if args.NumClientServers == 0 {
			clientFile = ""
		}
		net = coordinator.NewRemoteNetwork(args.ServerFile, args.GroupFile, clientFile)
		net.SetKill()
	}

	return net
}

func runExperiment(args ArgsCoordinator) {
	defer logger.Sugar.Sync()

	logger.Sugar.Info("Launching coordinator experiment")

	net := setupNetwork(args)
	if args.NetworkType == NETWORK_TYPE_LOCAL || args.NetworkType == NETWORK_TYPE_REMOTE {
		defer net.KillAll()
	}

	numLayers := args.NumLayers
	numServers := args.NumServers
	numMessages := args.NumUsers

	c := coordinator.NewCoordinator(net)
	if args.LoadMessages {
		c.LoadKeys(args.KeyFile)
		c.LoadMessages(args.MessageFile)
	}

	l := 0
	if !args.SkipPathGen {
		for i := 0; i < numLayers; i++ {
			exp := c.NewExperiment(i, numLayers, numServers, numMessages, args)
			if i == 0 {
				exp.KeyGen = !args.LoadMessages
				exp.LoadKeys = args.LoadMessages
			}
			exp.Info.PathEstablishment = true
			exp.Info.LastLayer = (i == numLayers-1)
			exp.Info.Check = !args.NoCheck
			exp.Info.Interval = int64(args.Interval)

			if args.BinSize > 0 {
				exp.Info.BinSize = int64(args.BinSize)
			} else if i == 0 {
				logger.Sugar.Infof("Using bin size %d", exp.Info.BinSize)
			}

			if args.LimitSize > 0 {
				exp.Info.BoomerangLimit = int64(args.LimitSize)
			} else {
				exp.Info.BoomerangLimit = int64(numLayers)
			}

			exp.Info.ReceiptLayer = 0
			if i-int(exp.Info.BoomerangLimit) > 0 {
				exp.Info.ReceiptLayer = int64(i) - exp.Info.BoomerangLimit
			}

			exp.Info.NextLayer = int64(i)

			err := c.DoAction(exp)
			if err != nil {
				logger.Sugar.Error(err)
				return
			}
			logger.Sugar.Infow("Path establishment round complete",
				"round", i,
				"time", time.Since(exp.ExperimentStartTime),
			)
			exp.RecordToFile(args.OutFile)
		}
		l = numLayers
	}

	numLightning := 5
	for i := l; i < l+numLightning; i++ {
		exp := c.NewExperiment(i, numLayers, numServers, numMessages, args)
		exp.Info.PathEstablishment = false
		exp.Info.MessageSize = int64(args.MessageSize)
		exp.Info.Check = !args.NoCheck
		if args.BinSize > 0 {
			exp.Info.BinSize = int64(args.BinSize)
		}
		if args.SkipPathGen && (i == 0) {
			exp.Info.SkipPathGen = true
			exp.KeyGen = true
		}
		exp.Info.Interval = int64(args.Interval)
		err := c.DoAction(exp)
		if err != nil {
			logger.Sugar.Error(err)
			return
		}
		logger.Sugar.Infow("Lightning round complete",
			"round", i,
			"time", time.Since(exp.ExperimentStartTime),
		)
		exp.RecordToFile(args.OutFile)
	}
}

func runMixnet(args ArgsCoordinator) {
	defer logger.Sugar.Sync()

	logger.Sugar.Info("Launching coordinator mix-net")

	// setup gateway and start proxy if enabled
	if args.MessageSize <= int(gateway.GetMaxProtocolSize()) {
		logger.Sugar.Fatal("Error: MessageSize too small for Gateway packet protocol")
	}
	gateway.Init(
		int64(args.MessageSize),
		args.GatewayEnable,
		args.GatewayAddrIn,
		args.GatewayAddrOut,
	)

	// setup network
	net := setupNetwork(args)
	if args.NetworkType == NETWORK_TYPE_LOCAL {
		defer net.KillAll()
	}

	numLayers := args.NumLayers
	numServers := args.NumServers
	numMessages := args.NumUsers

	c := coordinator.NewCoordinator(net)
	if args.LoadMessages {
		c.LoadKeys(args.KeyFile)
		c.LoadMessages(args.MessageFile)
	}

	//////////////////////////////////////////////////////
	// run rounds for each layer to establish paths
	//////////////////////////////////////////////////////
	for i := 0; i < numLayers; i++ {
		exp := c.NewExperiment(i, numLayers, numServers, numMessages, args)
		if i == 0 {
			exp.KeyGen = !args.LoadMessages
			exp.LoadKeys = args.LoadMessages
		}
		exp.Info.PathEstablishment = true
		exp.Info.LastLayer = (i == numLayers-1)
		exp.Info.Check = !args.NoCheck
		exp.Info.Interval = int64(args.Interval)

		if args.BinSize > 0 {
			exp.Info.BinSize = int64(args.BinSize)
		} else if i == 0 {
			logger.Sugar.Infof("Using bin size %d", exp.Info.BinSize)
		}

		if args.LimitSize > 0 {
			exp.Info.BoomerangLimit = int64(args.LimitSize)
		} else {
			exp.Info.BoomerangLimit = int64(numLayers)
		}

		exp.Info.ReceiptLayer = 0
		if i-int(exp.Info.BoomerangLimit) > 0 {
			exp.Info.ReceiptLayer = int64(i) - exp.Info.BoomerangLimit
		}

		exp.Info.NextLayer = int64(i)

		err := c.DoAction(exp)
		if err != nil {
			logger.Sugar.Error(err)
			return
		}
		logger.Sugar.Infow("Path establishment round complete",
			"round", i,
			"time", time.Since(exp.ExperimentStartTime),
		)
		exp.RecordToFile(args.OutFile)
	}

	////////////////////////////////////////////////////////////////////////
	// wait for CTRL-C to exit, leave servers running
	// https://stackoverflow.com/a/18158859
	////////////////////////////////////////////////////////////////////////
	ctrl := make(chan os.Signal)
	signal.Notify(ctrl, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctrl
		logger.Sugar.Info("Exiting")
		os.Exit(1)
	}()
	logger.Sugar.Info("Coordinator running... CTRL-C to exit.")

	//////////////////////////////////////////////////////
	// continually run lightning rounds to transmit messages
	//////////////////////////////////////////////////////
	var stats = utils.NewTimeStats()
	for i := numLayers; ; i++ {
		exp := c.NewExperiment(i, numLayers, numServers, numMessages, args)

		exp.Info.PathEstablishment = false
		exp.Info.MessageSize = int64(args.MessageSize)
		exp.Info.Check = !args.NoCheck

		if args.BinSize > 0 {
			exp.Info.BinSize = int64(args.BinSize)
		}

		if args.SkipPathGen && (i == 0) {
			exp.Info.SkipPathGen = true
			exp.KeyGen = true
		}

		exp.Info.Interval = int64(args.Interval)

		err := c.DoAction(exp)
		if err != nil {
			logger.Sugar.Error(err)
			return
		}

		stats.RecordTime(float64(time.Since(exp.ExperimentStartTime)))

		// print time stats
		c := 10 // print cycle, so terminal not too busy
		if i%c == 0 {
			logger.Sugar.Infow("Lightning round complete",
				"round", i,
				"stats", stats.GetStatsString(),
			)
		}

		exp.RecordToFile(args.OutFile)

		// sleep between rounds
		time.Sleep(time.Duration(args.RoundInterval) * time.Millisecond)
	}
}

// Note: This is a temporary solution within the evolving context of trellis
// simulator for servers to generate their own config files and only share
// the public aspects
// TODO: client servers
func runConfigGenerator(args ArgsCoordinator) {
	defer logger.Sugar.Sync()

	hosts := readHostsfile(args.Config.HostsFile)

	// hosts --> servers (to match trellis function expectations)
	remoteServers := make(map[int64]*config.Server)
	for i, host := range hosts {
		remoteServers[int64(i)] = &config.Server{
			Address: host,
		}
	}

	// run remote commands on each host to create their own private server config
	var wg sync.WaitGroup
	for _, s := range remoteServers {
		wg.Add(1)
		go func(s *config.Server) {
			defer wg.Done()
			cmd := fmt.Sprintf("xtrellis server config --addr %s", s.Address)
			if !coordinator.RunRemoteCommandOnEach(map[int64]*config.Server{int64(0): s}, cmd) {
				logger.Sugar.Fatalf("Could not run command `%s` on host %s", cmd, s.Address)
			}
		}(s)
	}
	wg.Wait()

	// create a temp dir to store public server config files
	tmpDir, err := os.MkdirTemp("", "xtrellis-")
	if err != nil {
		logger.Sugar.Fatalf("Could not create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// retrieve public server config from each host
	fn := getWorkingDirectory() + "/" + args.ServerPublicFile
	if !coordinator.TransferFileFromAllServers(remoteServers, fn, tmpDir) {
		logger.Sugar.Fatalf("Could not transfer file %s from all servers", fn)
	}

	// merge public server config from each host
	servers := make(map[int64]*config.Server)
	ids := make([]int64, 0)
	for id, host := range hosts {
		ids = append(ids, int64(id))
		s, err := config.UnmarshalServersFromFile(tmpDir + "/" + host + "-" + args.ServerPublicFile)
		if err != nil {
			logger.Sugar.Fatalf("Could not read servers file",
				"file", args.ServerPublicFile,
				"error", err,
			)
		}
		for _, s2 := range s {
			s2.Id = int64(id)
			servers[int64(id)] = s2
		}
	}

	err = config.MarshalServersToFile(args.ServerFile, servers)
	if err != nil {
		logger.Sugar.Fatalf("Could not write servers file",
			"file", args.ServerFile,
			"error", err,
		)
	}

	groups := config.CreateSeparateGroupsWithSize(args.NumGroups, args.GroupSize, ids)
	err = config.MarshalGroupsToFile(args.GroupFile, groups)
	if err != nil {
		logger.Sugar.Fatalf("Could not write group file",
			"file", args.GroupFile,
			"error", err,
		)
	}
}

func readHostsfile(fn string) []string {
	ifile, err := os.Open(fn)
	if err != nil {
		logger.Sugar.Fatalw("Could not open hosts file",
			"file", fn,
			"error", err,
		)
	}
	reader := csv.NewReader(ifile)
	hosts, err := reader.ReadAll()
	if err != nil {
		logger.Sugar.Fatalw("Could not read the list of hosts",
			"error", err,
		)
	}
	ifile.Close()

	output := make([]string, 0)
	for _, host := range hosts {
		output = append(output, host[0])
	}
	return output
}
