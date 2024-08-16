package main

import (
	"fmt"
	"os"

	"github.com/pedrogomes29/blockchain_wallet/cli"
)

func main() {
	serverURL := "http://localhost:8080"
	cliApp := cli.NewCLI(serverURL)

	rootCmd := cliApp.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
