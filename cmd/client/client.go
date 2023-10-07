package main

import (
	"flag"
	"log"

	//	"os"
	"fmt"

	"github.com/31333337/trellis/client"
	"github.com/31333337/trellis/config"
	"github.com/31333337/trellis/errors"
	"github.com/31333337/trellis/network"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s <serversFile> <groupsFile> <clientsFile> <addr>\n", flag.CommandLine.Name())
		fmt.Println("\nArguments:")
		fmt.Println("  serversFile: Path to the JSON file containing server configurations.")
		fmt.Println("  groupsFile: Path to the JSON file containing group configurations.")
		fmt.Println("  clientsFile: Path to the JSON file containing client configurations.")
		fmt.Println("  addr: Address for the client to bind to (e.g., localhost:8000).")
	}

	help := flag.Bool("help", false, "Display help")
	flag.Parse()

	if *help || len(flag.Args()) < 4 {
		flag.Usage()
		return
	}

	serversFile := flag.Arg(0)
	groupsFile := flag.Arg(1)
	clientsFile := flag.Arg(2)
	addr := flag.Arg(3)
	errors.Addr = addr
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
	err = clientRunner.Connect()
	if err != nil {
		log.Fatalf("Could not make clients %v", err)
	}
	network.RunServer(nil, clientRunner, clients, addr)
}
