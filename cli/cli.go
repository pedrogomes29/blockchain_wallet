package cli

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/pedrogomes29/blockchain_wallet/wallet"
	"github.com/spf13/cobra"
)

type CLI struct {
	rpcEndpoint string
	walletObj   *wallet.Wallet
}

func NewCLI() *CLI {
	cli := &CLI{}
	fmt.Println("Welcome to the wallet CLI!")
	cli.askForRPCEndpoint()
	cli.initializeWallet()
	return cli
}

func (cli *CLI) askForRPCEndpoint() {
	fmt.Print("Enter the RPC endpoint: ")
	var endpoint string
	fmt.Scanln(&endpoint)

	endpoint = strings.TrimSpace(endpoint)
	cli.rpcEndpoint = endpoint

	fmt.Printf("Using RPC endpoint: %s\n", cli.rpcEndpoint)
}

func (cli *CLI) initializeWallet() {
	fmt.Print("Do you want to create a new wallet or use an existing one? (new/existing): ")
	var choice string
	fmt.Scanln(&choice)
	choice = strings.TrimSpace(choice)

	if choice == "new" {
		fmt.Print("Enter a name for your new wallet: ")
		var name string
		fmt.Scanln(&name)

		wallet, err := wallet.NewWalletAndPrivateKey(name, cli.rpcEndpoint)
		if err != nil {
			log.Fatalf("Error creating wallet: %v", err)
		}
		cli.walletObj = wallet
		fmt.Println("New wallet created and saved.")
	} else if choice == "existing" {
		fmt.Print("Enter the name of your existing wallet: ")
		var name string
		fmt.Scanln(&name)

		wallet, err := wallet.NewWallet(name, cli.rpcEndpoint)
		if err != nil {
			log.Fatalf("Error loading wallet: %v", err)
		}
		cli.walletObj = wallet
		fmt.Println("Wallet loaded successfully.")
	} else {
		log.Fatalf("Invalid choice. Exiting...")
	}
}

func (cli *CLI) NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "wallet-cli",
		Short: "CLI for interacting with your blockchain wallet",
		Run:   cli.showMenu,
	}

	rootCmd.PersistentFlags().StringVar(&cli.rpcEndpoint, "RPC Endpoint", "http://localhost:8080", "RPC Endpoint")

	return rootCmd
}

func (cli *CLI) showMenu(cmd *cobra.Command, args []string) {
	for {
		fmt.Println("\nChoose an option:")
		fmt.Println("1. Address")
		fmt.Println("2. Balance")
		fmt.Println("3. Send")
		fmt.Println("4. Exit")

		var choice int
		fmt.Print("Enter your choice: ")
		_, err := fmt.Scan(&choice)
		if err != nil {
			log.Fatalf("Invalid input: %v", err)
		}

		switch choice {
		case 1:
			cli.showAddress()
		case 2:
			cli.showBalance()
		case 3:
			cli.sendCoins()
		case 4:
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid choice, please try again.")
		}
	}
}

func (cli *CLI) showAddress() {
	address := cli.walletObj.Address()
	fmt.Println("Your address:", address)
}

func (cli *CLI) showBalance() {
	balance := cli.walletObj.GetBalance()
	fmt.Println("Your current balance:", balance, "BTC")
}

func (cli *CLI) sendCoins() {
	fmt.Print("Enter the address to send to: ")
	var toAddress string
	fmt.Scanln(&toAddress)

	fmt.Print("Enter the amount to send: ")
	var amountStr string
	fmt.Scanln(&amountStr)

	amount, err := strconv.Atoi(amountStr)
	if err != nil {
		log.Fatalf("Invalid amount: %v", err)
	}

	err = cli.walletObj.SendToAddress(toAddress, amount)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Sent %d BTC to %s\n", amount, toAddress)
}
