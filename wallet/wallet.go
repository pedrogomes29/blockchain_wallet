package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/pedrogomes29/blockchain_node/transactions"
	"github.com/pedrogomes29/blockchain_node/utils"
)

const walletDir = "wallets"


type Wallet struct {
	privateKey ecdsa.PrivateKey
	serverURL  string
}

func NewWalletAndPrivateKey(walletName, serverURL string) (*Wallet, error) {
	privateKey := newPrivateKey()
	wallet := &Wallet{
		privateKey: privateKey,
		serverURL:  serverURL,
	}

	if err := wallet.savePrivateKey(walletName); err != nil {
		return nil, err
	}

	return wallet, nil
}

func NewWallet(walletName, serverURL string) (*Wallet, error) {
	privateKey, err := loadPrivateKey(walletName)
	if err != nil {
		return nil, err
	}

	return &Wallet{
		privateKey: *privateKey,
		serverURL:  serverURL,
	}, nil
}

func newPrivateKey() ecdsa.PrivateKey {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}
	return *private
}

func (wallet *Wallet) PublicKey() []byte {
	privateKey := wallet.privateKey
	publicKey := privateKey.PublicKey

	return append(publicKey.X.Bytes(), publicKey.Y.Bytes()...)
}

func (wallet *Wallet) PublicKeyHash() []byte {
	return utils.HashPublicKey(wallet.PublicKey())
}

func (wallet *Wallet) Address() string {
	pubKeyHash := wallet.PublicKeyHash()
	return base58.CheckEncode(pubKeyHash, 0x00)
}

func (wallet *Wallet) generateTxToAddress(toAddress string, amount int) *transactions.Transaction {
	var inputs []transactions.TXInput
	var outputs []transactions.TXOutput
	inputIdxsToSign := make(map[int]struct{})

	publicKeyHash := wallet.PublicKeyHash()
	utxosTotal, spendableUTXOs, err := wallet.findSpendableUTXOs(publicKeyHash, amount)
	if err != nil {
		log.Panic(err)
	}
	if utxosTotal < amount {
		log.Panic("ERROR: Not enough funds")
	}

	for txHashString, outs := range spendableUTXOs {
		txHash, err := hex.DecodeString(txHashString)
		if err != nil {
			log.Panic(err)
		}

		for _, out := range outs {
			input := transactions.TXInput{
				Txid:     txHash,
				OutIndex: out,
				Signature: nil,
				PubKey:   wallet.PublicKey(),
			}
			inputs = append(inputs, input)
			inputIdxsToSign[len(inputs)-1] = struct{}{}
		}
	}

	transactionToAddress, err := transactions.NewTXOutput(amount, toAddress)
	if err != nil {
		log.Panic(err)
	}
	outputs = append(outputs, *transactionToAddress)

	if utxosTotal > amount {
		transactionChange, err := transactions.NewTXOutput(utxosTotal-amount, wallet.Address())
		if err != nil {
			log.Panic(err)
		}
		outputs = append(outputs, *transactionChange)
	}

	transaction := &transactions.Transaction{
		ID:         nil,
		Vin:        inputs,
		Vout:       outputs,
		IsCoinbase: false,
	}
	transaction.ID = transaction.Hash()

	wallet.SignTransactionInputs(transaction, inputIdxsToSign)

	return transaction
}

func (wallet *Wallet) SendToAddress(toAddress string, amount int) {
	tx := wallet.generateTxToAddress(toAddress, amount)
	wallet.sendTransaction(tx)
}

func (wallet *Wallet) GetBalance() int {
	UTXOs, err := wallet.findUTXOs(wallet.PublicKeyHash())
	if err != nil {
		log.Panic(err)
	}
	balance := 0
	for _, out := range UTXOs {
		balance += out.Value
	}
	return balance
}

func (wallet *Wallet) SignTransactionInputs(tx *transactions.Transaction, inputIdxsToSign map[int]struct{}) {
	txCopy := tx.TrimmedCopy()
	for inputIdxToSign := range inputIdxsToSign {
		r, s, err := ecdsa.Sign(rand.Reader, &wallet.privateKey, txCopy.Hash())
		if err != nil {
			log.Panic(err)
		}
		signature := append(r.Bytes(), s.Bytes()...)
		tx.Vin[inputIdxToSign].Signature = signature
	}
}

func (wallet *Wallet) findUTXOs(pubKeyHash []byte) ([]transactions.TXOutput, error) {
	pubKeyHashStr := hex.EncodeToString(pubKeyHash)
	url := fmt.Sprintf("%s/utxos?pubKeyHash=%s", wallet.serverURL, pubKeyHashStr)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error contacting server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve UTXOs: %s", resp.Status)
	}

	var utxos []transactions.TXOutput
	if err := json.NewDecoder(resp.Body).Decode(&utxos); err != nil {
		return nil, fmt.Errorf("error decoding server response: %v", err)
	}
	return utxos, nil
}

func (wallet *Wallet) findSpendableUTXOs(pubKeyHash []byte, amount int) (int, map[string][]int, error) {
	pubKeyHashStr := hex.EncodeToString(pubKeyHash)
	amountStr := strconv.Itoa(amount)
	url := fmt.Sprintf("%s/spendable_utxos?pubKeyHash=%s&amount=%s", wallet.serverURL, pubKeyHashStr, amountStr)

	resp, err := http.Get(url)
	if err != nil {
		return 0, nil, fmt.Errorf("error contacting server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var result struct {
		Total     int               `json:"total"`
		Spendable map[string][]int  `json:"spendable"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, nil, fmt.Errorf("error decoding server response: %v", err)
	}

	return result.Total, result.Spendable, nil
}

func (wallet *Wallet) sendTransaction(tx *transactions.Transaction) error {
	url := fmt.Sprintf("%s/transaction", wallet.serverURL)
	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("error marshalling transaction: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("error sending transaction: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error: %s", string(body))
	}

	return nil
}

func encodePrivateKey(privateKey *ecdsa.PrivateKey) (string, error){
    x509Encoded, err := x509.MarshalECPrivateKey(privateKey)
	if err!=nil{
		return "",err
	}
    pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})


    return string(pemEncoded),nil
}

func decodePrivateKey(pemEncoded string) (*ecdsa.PrivateKey,error) {
    block, _ := pem.Decode([]byte(pemEncoded))
    x509Encoded := block.Bytes
    privateKey, err := x509.ParseECPrivateKey(x509Encoded)
	if err!=nil{
		return nil,err
	}
    return privateKey,nil
}


func (wallet *Wallet) savePrivateKey(walletName string) error {
	privateKeyStr, err := encodePrivateKey(&wallet.privateKey)
	if err != nil {
		return fmt.Errorf("error encoding private key: %v", err)
	}

	keyData := map[string]string{
		"privatekey": privateKeyStr,
	}
	jsonData, err := json.MarshalIndent(keyData, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling private key to JSON: %v", err)
	}

	err = os.MkdirAll(walletDir, 0700)
	if err != nil {
		return fmt.Errorf("error creating wallet directory: %v", err)
	}

	filePath := filepath.Join(walletDir, walletName+".json")
	err = os.WriteFile(filePath, jsonData, 0600)
	if err != nil {
		return fmt.Errorf("error saving private key to file: %v", err)
	}

	return nil
}

func loadPrivateKey(walletName string) (*ecdsa.PrivateKey, error) {
	filePath := filepath.Join(walletDir, walletName+".json")
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file: %v", err)
	}

	var keyData map[string]string
	err = json.Unmarshal(jsonData, &keyData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON data: %v", err)
	}

	privateKeyStr, ok := keyData["privatekey"]
	if !ok {
		return nil, fmt.Errorf("private key not found in JSON data")
	}

	privateKey, err := decodePrivateKey(privateKeyStr)
	if err != nil {
		return nil, fmt.Errorf("error decoding private key: %v", err)
	}

	return privateKey, nil
}