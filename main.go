package main

import (
	"fmt"
	"os"

	"github.com/pedrogomes29/blockchain_wallet/cli"
)

func main() {
	cliApp := cli.NewCLI()

	rootCmd := cliApp.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
