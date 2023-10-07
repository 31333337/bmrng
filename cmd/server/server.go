package main

import (
	"fmt"
	"log"

	//	"os"
	"flag"

	"https://github.com/31333337/trellis/config"
	"https://github.com/31333337/trellis/errors"
	"https://github.com/31333337/trellis/network"
	"https://github.com/31333337/trellis/server"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s <serversFile> <groupsFile> <addr>\n", flag.CommandLine.Name())
		fmt.Println("\nArguments:")
		fmt.Println("  serversFile: Path to the JSON file containing server configurations.")
		fmt.Println("  groupsFile: Path to the JSON file containing group configurations.")
		fmt.Println("  addr: Address for the server to bind to (e.g., localhost:8000).")
	}

	help := flag.Bool("help", false, "Display help")
	flag.Parse()

	if *help || len(flag.Args()) < 3 {
		flag.Usage()
		return
	}

	serversFile := flag.Arg(0)
	groupsFile := flag.Arg(1)
	addr := flag.Arg(2)
	errors.Addr = addr
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
	// f, err := os.Create("path.pprof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()
	server.TcpConnections.LaunchAccepts()
	network.RunServer(h, server, servers, addr)
	config.Flush()
}
