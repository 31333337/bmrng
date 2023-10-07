package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	arg "github.com/alexflint/go-arg"
	"https://github.com/31333337/trellis/client"
	"https://github.com/31333337/trellis/config"
	"https://github.com/31333337/trellis/coordinator"
	"https://github.com/31333337/trellis/errors"
	"https://github.com/31333337/trellis/network"
	"https://github.com/31333337/trellis/server"
	"https://github.com/31333337/trellis/xtrellis/gateway"
	"https://github.com/31333337/trellis/xtrellis/utils"
)

type Args struct {
	Mode  string `arg:"positional,required" help:"execution mode: coordinator, server, or client"`
	Debug bool   `default:"False" help:"enable debug log output"`

	////////////////////////////////////
	// files
	////////////////////////////////////
	ServerFile  string `default:"servers.json"`
	GroupFile   string `default:"groups.json"`
	ClientFile  string `default:"clients.json"`
	KeyFile     string `default:"keys.json"`
	MessageFile string `default:"messages.json"`
	OutFile     string `default:"res.json"`

	////////////////////////////////////
	// client & server
	////////////////////////////////////
	Addr string `default:"localhost:8000"`

	////////////////////////////////////
	// coordinator
	////////////////////////////////////
	GatewayAddrIn  string `default:"localhost:9000" help:"gateway proxy address for incoming mix-net messages"`
	GatewayAddrOut string `default:"localhost:9900" help:"gateway proxy address for outgoing mix-net messages"`
	GatewayEnable  bool   `default:"False" help:"enable client message gateway"`
	RoundInterval  int    `default:"0" help:"delay (in ms) between mix-net lightning rounds"`
	RunExperiment  bool   `default:"False" help:"run coordinator experiment"`

	F           float64 `default:"0"`
	RunType     int     `default:"1"`
	NumUsers    int     `default:"100" help:"also NumMessages"`
	NumServers  int     `default:"10"`
	MessageSize int     `default:"1024"`

	NumGroups int `default:"3"`
	GroupSize int `default:"3"`
	BinSize   int `default:"0"`
	LimitSize int `default:"0"`
	NumLayers int `default:"10"`
	Overflow  int `default:"32"`

	NumClientServers int  `default:"0"`
	SkipPathGen      bool `default:"False"`
	NoCheck          bool `default:"False"`
	LoadMessages     bool `default:"False"`
	StartIdx         int  `default:"0"`
	Interval         int  `default:"0"`
}

func LaunchCoordinator(args Args) {
	p := arg.MustParse(&args)

	////////////////////////////////////////////////////////////////////////
	// process args
	////////////////////////////////////////////////////////////////////////

	if args.GroupSize == 0 {
		if args.F != 0 {
			if args.NumGroups != 0 {
				args.GroupSize = config.CalcGroupSize(args.NumServers, args.NumGroups, args.F)
			} else {
				args.GroupSize, args.NumGroups = config.CalcFewGroups2(args.F, args.NumServers)
			}
		} else {
			log.Printf("Set groupsize or f")
			p.WriteHelp(os.Stdout)
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
			log.Printf("Set numlayers or f")
			p.WriteHelp(os.Stdout)
			return
		}
	}

	if args.BinSize == 0 {
		args.BinSize = config.BinSize2(args.NumLayers, args.NumServers, args.NumUsers, -args.Overflow)
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

	log.Printf("%+v", args)

	////////////////////////////////////////////////////////////////////////
	// setup gateway and start proxy if enabled
	////////////////////////////////////////////////////////////////////////
	if args.MessageSize <= int(gateway.GetMaxProtocolSize()) {
		log.Fatal("Error: MessageSize too small for Gateway packet protocol")
	}

	gateway.Init(int64(args.MessageSize), args.GatewayEnable, args.GatewayAddrIn, args.GatewayAddrOut)

	////////////////////////////////////////////////////////////////////////
	// setup network
	////////////////////////////////////////////////////////////////////////
	var net *coordinator.CoordinatorNetwork

	switch args.RunType {
	case 0: // run in the same process
		net = coordinator.NewInProcessNetwork(args.NumServers, args.NumGroups, args.GroupSize)

	case 1: // run in separate process on the same machine
		serverConfigs, groupConfigs, clientConfigs := coordinator.NewLocalConfig(args.NumServers, args.NumGroups, args.GroupSize, args.NumClientServers, false)
		if args.LoadMessages {
			oldServers, err := config.UnmarshalServersFromFile(args.ServerFile)
			if err != nil {
				log.Fatalf("Could not read servers file %s", args.ServerFile)
			}
			// copy old keys
			for id, s := range serverConfigs {
				old := oldServers[id]
				s.PrivateKey = old.PrivateKey
				s.PublicKey = old.PublicKey
			}
		}
		net = coordinator.NewLocalNetwork(serverConfigs, groupConfigs, clientConfigs)
		defer net.KillAll()
	}

	////////////////////////////////////////////////////////////////////////
	// run experiment
	////////////////////////////////////////////////////////////////////////
	if args.RunExperiment {
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
				log.Printf("\n")
				log.Printf("Round %v", i)
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
					log.Printf("Using bin size %d", exp.Info.BinSize)
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
					log.Print(err)
					return
				}
				log.Printf("Path round %v took %v", i, time.Since(exp.ExperimentStartTime))
				exp.RecordToFile(args.OutFile)
			}
			l = numLayers
		}

		numLightning := 5
		for i := l; i < l+numLightning; i++ {
			log.Printf("\n")
			log.Printf("Round %v", i)
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
				log.Print(err)
				return
			}
			log.Printf("Lightning round %v took %v", i, time.Since(exp.ExperimentStartTime))
			exp.RecordToFile(args.OutFile)
		}
		l += numLightning
	}

	////////////////////////////////////////////////////////////////////////
	// run mix-net
	////////////////////////////////////////////////////////////////////////
	if !args.RunExperiment {
		log.Printf("Running mix-net...")

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
				log.Printf("Using bin size %d", exp.Info.BinSize)
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
				log.Print(err)
				return
			}
			log.Printf("Path round %v took %v", i, time.Since(exp.ExperimentStartTime))
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
			log.Println("Exiting")
			os.Exit(1)
		}()
		log.Println("Coordinator running... CTRL-C to exit.")

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
				log.Print(err)
				return
			}

			stats.RecordTime(float64(time.Since(exp.ExperimentStartTime)))

			// print time stats
			c := 10 // print cycle, so terminal not too busy
			if i%c == 0 {
				log.Printf("Lightning round %v : %s", i, stats.GetStatsString())
			}

			exp.RecordToFile(args.OutFile)

			// sleep between rounds
			time.Sleep(time.Duration(args.RoundInterval) * time.Millisecond)
		}
	}
}

// from cmd/server/server.go
func LaunchServer(args Args) {
	arg.MustParse(&args)

	serversFile := args.ServerFile
	groupsFile := args.GroupFile
	addr := args.Addr
	errors.Addr = addr

	log.Printf("Launching server with address %s", addr)

	servers, err := config.UnmarshalServersFromFile(serversFile)
	if err != nil {
		log.Fatalf("Could not read servers file %s", serversFile)
	}

	groups, err := config.UnmarshalGroupsFromFile(groupsFile)
	if err != nil {
		log.Fatalf("Could not read group file %s", groupsFile)
	}

	// will start in blocked state
	h := server.NewHandler()
	server := server.NewServer(&config.Servers{Servers: servers}, &config.Groups{Groups: groups}, h, addr)

	server.TcpConnections.LaunchAccepts()
	network.RunServer(h, server, servers, addr)
	config.Flush()
}

// from cmd/client/client.go
func LaunchClient(args Args) {
	arg.MustParse(&args)

	serversFile := args.ServerFile
	groupsFile := args.GroupFile
	clientsFile := args.ClientFile
	addr := args.Addr
	errors.Addr = addr

	log.Printf("Launching client with address %s", addr)

	servers, err := config.UnmarshalServersFromFile(serversFile)
	if err != nil {
		log.Fatalf("Could not read servers file %s", serversFile)
	}

	groups, err := config.UnmarshalGroupsFromFile(groupsFile)
	if err != nil {
		log.Fatalf("Could not read group file %s", groupsFile)
	}

	clients, err := config.UnmarshalServersFromFile(clientsFile)
	if err != nil {
		log.Fatalf("Could not read clients file %s", clientsFile)
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

func main() {
	var args Args
	p := arg.MustParse(&args)

	utils.SetDebugLogEnabled(args.Debug)

	switch args.Mode {
	case "coordinator":
		LaunchCoordinator(args)

	case "client":
		LaunchClient(args)

	case "server":
		LaunchServer(args)

	default:
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}

}
