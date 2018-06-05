package main

import (
	"log"
	"strings"

	"context"

	"fmt"

	"os"

	"crypto/ecdsa"

	"math/big"

	"sync"

	"time"

	"github.com/SmartMeshFoundation/SmartRaiden"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc/contracts"
	"github.com/SmartMeshFoundation/SmartRaiden/params"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/huamou/config"
	"gopkg.in/urfave/cli.v1"
)

var globalPassword = "123"
var env, _ = config.ReadDefault("../env.INI")

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "keystore-path",
			Usage: "If you have a non-standard path for the ethereum keystore directory provide it using this argument. ",
			Value: "../../../../testdata/keystore",
		},
		cli.StringFlag{
			Name: "eth-rpc-endpoint",
			Usage: `"host:port" address of ethereum JSON-RPC server.\n'
	           'Also accepts a protocol prefix (ws:// or ipc channel) with optional port',`,
			Value: fmt.Sprintf("http://127.0.0.1:8545"), //, node.DefaultWSEndpoint()),
		},
		cli.BoolFlag{
			Name:  "not-create-channel",
			Usage: "not-create channels between node for test.",
		},
	}
	app.Action = Main
	app.Name = "envinit"
	app.Version = "0.1"
	app.Run(os.Args)
}

// Main : main
func Main(ctx *cli.Context) error {
	paramsSection := "RAIDEN_PARAMS"
	fmt.Printf("eth-rpc-endpoint:%s\n", ctx.String("eth-rpc-endpoint"))
	fmt.Printf("not-create-channel=%v\n", ctx.Bool("not-create-channel"))
	// Create an IPC based RPC connection to a remote node and an authorized transactor
	conn, err := ethclient.Dial(ctx.String("eth-rpc-endpoint"))
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}

	_, key := promptAccount(ctx.String("keystore-path"))
	fmt.Println("start to deploy ...")
	registryAddress := DeployContract(key, conn)
	env.RemoveOption(paramsSection, "registry_contract_address")
	env.AddOption(paramsSection, "registry_contract_address", registryAddress.String())
	//registryAddress := common.HexToAddress("0x7CCBe22b9A5edCc87163EF3014277F027d542D39")
	registry, err := contracts.NewRegistry(registryAddress, conn)
	if err != nil {
		return err
	}
	createTokenAndChannels(key, conn, registry, ctx.String("keystore-path"), !ctx.Bool("not-create-channel"))
	createTokenAndChannels(key, conn, registry, ctx.String("keystore-path"), !ctx.Bool("not-create-channel"))
	env.WriteFile("../env.INI", 0644, "smartraiden smoke test envInit")
	return nil
}
func promptAccount(keystorePath string) (addr common.Address, key *ecdsa.PrivateKey) {
	am := smartraiden.NewAccountManager(keystorePath)
	if len(am.Accounts) == 0 {
		log.Fatal(fmt.Sprintf("No Ethereum accounts found in the directory %s", keystorePath))
		os.Exit(1)
	}
	// write accounts to the env.INI
	env.RemoveSection("ACCOUNT")
	env.AddSection("ACCOUNT")
	for index, account := range am.Accounts {
		env.AddOption("ACCOUNT", fmt.Sprintf("N%d", index), account.Address.String())
	}
	addr = am.Accounts[0].Address
	for i := 0; i < 3; i++ {
		//retries three times
		if len(globalPassword) <= 0 {
			fmt.Printf("Enter the password to unlock")
			fmt.Scanln(&globalPassword)
		}
		//fmt.Printf("\npassword is %s\n", password)
		keybin, err := am.GetPrivateKey(addr, globalPassword)
		if err != nil && i == 3 {
			log.Fatal(fmt.Sprintf("Exhausted passphrase unlock attempts for %s. Aborting ...", addr))
			os.Exit(1)
		}
		if err != nil {
			log.Println(fmt.Sprintf("password incorrect\n Please try again or kill the process to quit.\nUsually Ctrl-c."))
			continue
		}
		key, err = crypto.ToECDSA(keybin)
		if err != nil {
			log.Println(fmt.Sprintf("private key to bytes err %s", err))
		}
		break
	}
	return
}

// DeployContract :
func DeployContract(key *ecdsa.PrivateKey, conn *ethclient.Client) (RegistryAddress common.Address) {
	auth := bind.NewKeyedTransactor(key)
	//DeployNettingChannelLibrary
	NettingChannelLibraryAddress, tx, _, err := contracts.DeployNettingChannelLibrary(auth, conn)
	if err != nil {
		log.Fatalf("Failed to DeployNettingChannelLibrary: %v", err)
	}
	ctx := context.Background()
	_, err = bind.WaitDeployed(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to deploy contact when mining :%v", err)
	}
	fmt.Printf("DeployNettingChannelLibrary complete...\n")
	//DeployChannelManagerLibrary link nettingchannle library before deploy
	contracts.ChannelManagerLibraryBin = strings.Replace(contracts.ChannelManagerLibraryBin, "__NettingChannelLibrary.sol:NettingCha__", NettingChannelLibraryAddress.String()[2:], -1)
	ChannelManagerLibraryAddress, tx, _, err := contracts.DeployChannelManagerLibrary(auth, conn)
	if err != nil {
		log.Fatalf("Failed to deploy new token contract: %v", err)
	}
	ctx = context.Background()
	_, err = bind.WaitDeployed(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to deploy contact when mining :%v", err)
	}
	fmt.Printf("DeployChannelManagerLibrary complete...\n")
	//DeployRegistry link channelmanagerlibrary before deploy
	contracts.RegistryBin = strings.Replace(contracts.RegistryBin, "__ChannelManagerLibrary.sol:ChannelMan__", ChannelManagerLibraryAddress.String()[2:], -1)
	RegistryAddress, tx, _, err = contracts.DeployRegistry(auth, conn)
	if err != nil {
		log.Fatalf("Failed to deploy new token contract: %v", err)
	}
	ctx = context.Background()
	_, err = bind.WaitDeployed(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to deploy contact when mining :%v", err)
	}
	fmt.Printf("DeployRegistry complete...\n")
	//EndpointRegistryAddress, tx, _, err := contracts.DeployEndpointRegistry(auth, conn)
	//if err != nil {
	//	log.Fatalf("Failed to deploy new token contract: %v", err)
	//}
	//env.RemoveOption("RAIDEN_PARAMS", "discovery_contract_address")
	//env.AddOption("RAIDEN_PARAMS", "discovery_contract_address", EndpointRegistryAddress.String())
	//ctx = context.Background()
	//_, err = bind.WaitDeployed(ctx, conn, tx)
	//if err != nil {
	//	log.Fatalf("failed to deploy contact when mining :%v", err)
	//}
	fmt.Printf("RegistryAddress=%s\n", RegistryAddress.String())
	return
}
func createTokenAndChannels(key *ecdsa.PrivateKey, conn *ethclient.Client, registry *contracts.Registry, keystorepath string, createchannel bool) {
	managerAddress, tokenAddress := NewToken(key, conn, registry)
	//tokenAddress := common.HexToAddress("0xD29A9Cbf2Ca88981D0794ce94e68495c4bC16F28")
	//managerAddress, _ := registry.ChannelManagerByToken(nil, tokenAddress)
	manager, err := contracts.NewChannelManagerContract(managerAddress, conn)
	if err != nil {
		log.Fatalf("err for NewChannelManagerContract %s", err)
		return
	}
	token, err := contracts.NewToken(tokenAddress, conn)
	if err != nil {
		log.Fatalf("err for newtoken err %s", err)
		return
	}
	am := smartraiden.NewAccountManager(keystorepath)
	var accounts []common.Address
	var keys []*ecdsa.PrivateKey
	for _, account := range am.Accounts {
		accounts = append(accounts, account.Address)
		keybin, err := am.GetPrivateKey(account.Address, globalPassword)
		if err != nil {
			log.Fatalf("password error for %s", account.Address.String())
		}
		keytemp, err := crypto.ToECDSA(keybin)
		if err != nil {
			log.Fatalf("toecdsa err %s", err)
		}
		keys = append(keys, keytemp)
	}
	fmt.Printf("key=%s", key)
	TransferMoneyForAccounts(key, conn, accounts[1:], token)
	if createchannel {
		CreateChannels(conn, accounts, keys, manager, token)
	}
}

// NewToken ：
func NewToken(key *ecdsa.PrivateKey, conn *ethclient.Client, registry *contracts.Registry) (mgrAddress common.Address, tokenAddr common.Address) {
	auth := bind.NewKeyedTransactor(key)
	tokenAddr, tx, _, err := contracts.DeployHumanStandardToken(auth, conn, big.NewInt(50000000000), "test", 2, "test symoble")
	if err != nil {
		log.Fatalf("Failed to DeployHumanStandardToken: %v", err)
	}
	fmt.Printf("token deploy tx=%s\n", tx.Hash().String())
	ctx := context.Background()
	_, err = bind.WaitDeployed(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to deploy contact when mining :%v", err)
	}
	fmt.Printf("DeployHumanStandardToken complete...\n")
	tx, err = registry.AddToken(auth, tokenAddr)
	if err != nil {
		log.Fatalf("Failed to AddToken: %v", err)
	}
	ctx = context.Background()
	_, err = bind.WaitMined(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to AddToken when mining :%v", err)
	}
	mgrAddress, err = registry.ChannelManagerByToken(nil, tokenAddr)
	fmt.Printf("DeployHumanStandardToken complete... %s,mgr=%s\n", tokenAddr.String(), mgrAddress.String())
	return
}

// TransferMoneyForAccounts :
func TransferMoneyForAccounts(key *ecdsa.PrivateKey, conn *ethclient.Client, accounts []common.Address, token *contracts.Token) {
	wg := sync.WaitGroup{}
	wg.Add(len(accounts))
	auth := bind.NewKeyedTransactor(key)
	nonce, _ := conn.PendingNonceAt(context.Background(), auth.From)
	for index, account := range accounts {
		go func(account common.Address, i int) {
			auth2 := bind.NewKeyedTransactor(key)
			auth2.Nonce = big.NewInt(int64(nonce) + int64(i))
			fmt.Printf("transfer to %s,nonce=%s\n", account.String(), auth2.Nonce)
			tx, err := token.Transfer(auth2, account, big.NewInt(5000000))
			if err != nil {
				log.Fatalf("Failed to Transfer: %v", err)
			}
			ctx := context.Background()
			_, err = bind.WaitMined(ctx, conn, tx)
			if err != nil {
				log.Fatalf("failed to Transfer when mining :%v", err)
			}
			fmt.Printf("Transfer complete...\n")
			wg.Done()
		}(account, index)
		time.Sleep(time.Millisecond * 100)
	}
	wg.Wait()
	for _, account := range accounts {
		b, _ := token.BalanceOf(nil, account)
		log.Printf("account %s has token %s\n", utils.APex(account), b)
	}
}

// CreateChannels : path A-B-C-F-B-D-G-E
func CreateChannels(conn *ethclient.Client, accounts []common.Address, keys []*ecdsa.PrivateKey, manager *contracts.ChannelManagerContract, token *contracts.Token) {
	if len(accounts) < 6 {
		panic("need 6 accounts")
	}
	AccountA := accounts[0]
	AccountB := accounts[1]
	AccountC := accounts[2]
	AccountD := accounts[3]
	AccountE := accounts[4]
	AccountF := accounts[5]
	AccountG := accounts[6]
	fmt.Printf("accountA=%s\naccountB=%s\naccountC=%s\naccountD=%s\naccountE=%s\naccountF=%s\naccountG=%s\n",
		AccountA.String(), AccountB.String(), AccountC.String(), AccountD.String(),
		AccountE.String(), AccountF.String(), AccountG.String())
	keyA := keys[0]
	keyB := keys[1]
	keyC := keys[2]
	keyD := keys[3]
	keyE := keys[4]
	keyF := keys[5]
	keyG := keys[6]
	fmt.Printf("keya=%s,keyb=%s,keyc=%s,keyd=%s,keye=%s,keyf=%s,keyg=%s", keyA, keyB, keyC, keyD, keyE, keyF, keyG)
	creatAChannelAndDeposit(AccountA, AccountB, keyA, keyB, 100, manager, token, conn)
	creatAChannelAndDeposit(AccountB, AccountD, keyB, keyD, 90, manager, token, conn)
	creatAChannelAndDeposit(AccountB, AccountC, keyB, keyC, 50, manager, token, conn)
	creatAChannelAndDeposit(AccountB, AccountF, keyB, keyF, 70, manager, token, conn)
	creatAChannelAndDeposit(AccountC, AccountF, keyC, keyF, 60, manager, token, conn)
	creatAChannelAndDeposit(AccountC, AccountE, keyC, keyE, 10, manager, token, conn)
	creatAChannelAndDeposit(AccountD, AccountG, keyD, keyG, 90, manager, token, conn)
	creatAChannelAndDeposit(AccountG, AccountE, keyG, keyE, 80, manager, token, conn)

}

// CreateChannels2 :
func CreateChannels2(conn *ethclient.Client, accounts []common.Address, keys []*ecdsa.PrivateKey, manager *contracts.ChannelManagerContract, token *contracts.Token) {
	if len(accounts) < 6 {
		panic("need 6 accounts")
	}
	AccountA := accounts[0]
	AccountB := accounts[1]
	AccountC := accounts[2]
	AccountD := accounts[3]
	AccountE := accounts[4]
	AccountF := accounts[5]
	fmt.Printf("accountA=%saccountB=%saccountC=%saccountD=%saccountE=%saccountF=%s\n", AccountA.String(), AccountB.String(), AccountC.String(), AccountD.String(), AccountE.String(), AccountF.String())
	keyA := keys[0]
	keyB := keys[1]
	keyC := keys[2]
	keyD := keys[3]
	keyE := keys[4]
	keyF := keys[5]
	fmt.Printf("keya=%s,keyb=%s,keyc=%s,keyd=%s,keye=%s,keyf=%s", keyA, keyB, keyC, keyD, keyE, keyF)
	creatAChannelAndDeposit(AccountA, AccountB, keyA, keyB, 100, manager, token, conn)
	creatAChannelAndDeposit(AccountB, AccountD, keyB, keyD, 50, manager, token, conn)
	creatAChannelAndDeposit(AccountA, AccountC, keyA, keyC, 90, manager, token, conn)
	creatAChannelAndDeposit(AccountC, AccountE, keyC, keyE, 80, manager, token, conn)
	creatAChannelAndDeposit(AccountE, AccountD, keyE, keyD, 70, manager, token, conn)

}

/*
CreateChannels3 :
registry address :0x0C31cF985eA2F2932c2EDF05f36aBC7b24B17d40 test poa net networkid :8888
you find this topology at the above address
*/
func CreateChannels3(conn *ethclient.Client, accounts []common.Address, keys []*ecdsa.PrivateKey, manager *contracts.ChannelManagerContract, token *contracts.Token) {
	if len(accounts) < 6 {
		panic("need 6 accounts")
	}
	AccountA := accounts[0]
	AccountB := accounts[1]
	AccountC := accounts[2]
	AccountD := accounts[3]
	AccountE := accounts[4]
	AccountF := accounts[5]
	fmt.Printf("accountA=%saccountB=%saccountC=%saccountD=%saccountE=%saccountF=%s\n", AccountA.String(), AccountB.String(), AccountC.String(), AccountD.String(), AccountE.String(), AccountF.String())
	keyA := keys[0]
	keyB := keys[1]
	keyC := keys[2]
	keyD := keys[3]
	keyE := keys[4]
	keyF := keys[5]

	creatAChannelAndDeposit(AccountA, AccountB, keyA, keyB, 100, manager, token, conn)
	creatAChannelAndDeposit(AccountB, AccountC, keyB, keyC, 90, manager, token, conn)
	creatAChannelAndDeposit(AccountC, AccountD, keyC, keyD, 50, manager, token, conn)

	creatAChannelAndDeposit(AccountB, AccountE, keyB, keyE, 80, manager, token, conn)
	creatAChannelAndDeposit(AccountE, AccountF, keyE, keyF, 70, manager, token, conn)
	creatAChannelAndDeposit(AccountF, AccountD, keyF, keyD, 60, manager, token, conn)

}

// CreateChannels4 :
func CreateChannels4(conn *ethclient.Client, accounts []common.Address, keys []*ecdsa.PrivateKey, manager *contracts.ChannelManagerContract, token *contracts.Token) {
	if len(accounts) < 6 {
		panic("need 6 accounts")
	}
	AccountA := accounts[0]
	AccountB := accounts[1]
	AccountC := accounts[2]
	AccountD := accounts[3]
	AccountE := accounts[4]
	AccountF := accounts[5]
	fmt.Printf("accountA=%saccountB=%saccountC=%saccountD=%saccountE=%saccountF=%s\n", AccountA.String(), AccountB.String(), AccountC.String(), AccountD.String(), AccountE.String(), AccountF.String())
	keyA := keys[0]
	keyB := keys[1]
	keyC := keys[2]
	keyD := keys[3]
	keyE := keys[4]
	keyF := keys[5]
	/*
	   4.1 create channel A-B and save 100 both

	*/
	creatAChannelAndDeposit(AccountA, AccountB, keyA, keyB, 100, manager, token, conn)
	/*
	 4.2 create channel B-C and save 50 both
	*/
	creatAChannelAndDeposit(AccountB, AccountC, keyB, keyC, 50, manager, token, conn)
	/*
	  4.3 create channel C-E and save 100 both
	*/

	creatAChannelAndDeposit(AccountC, AccountE, keyC, keyE, 100, manager, token, conn)

	/*
	   4.4 create channel A-D and save 100 both
	*/

	creatAChannelAndDeposit(AccountA, AccountD, keyA, keyD, 100, manager, token, conn)

	/*
	  4.5 create channel B-D and save 100 both
	*/

	creatAChannelAndDeposit(AccountB, AccountD, keyB, keyD, 100, manager, token, conn)

	/*
	   4.6 create channel D-F and save 100 both
	*/

	creatAChannelAndDeposit(AccountD, AccountF, keyD, keyF, 100, manager, token, conn)

	/*
	   4.7 create channel F-E and save 100 both
	*/

	creatAChannelAndDeposit(AccountF, AccountE, keyF, keyE, 100, manager, token, conn)
	/*
	   4.8 create channel C-F and save 50 both
	*/

	creatAChannelAndDeposit(AccountC, AccountF, keyC, keyF, 50, manager, token, conn)

	/*
	   4.9     D-E 100
	*/

	creatAChannelAndDeposit(AccountD, AccountE, keyD, keyE, 100, manager, token, conn)

}
func creatAChannelAndDeposit(account1, account2 common.Address, key1, key2 *ecdsa.PrivateKey, amount int64, manager *contracts.ChannelManagerContract, token *contracts.Token, conn *ethclient.Client) {
	log.Printf("createchannel between %s-%s\n", utils.APex(account1), utils.APex(account2))
	auth1 := bind.NewKeyedTransactor(key1)
	auth1.GasLimit = uint64(params.GasLimit)
	auth1.GasPrice = big.NewInt(params.GasPrice)
	callAuth1 := &bind.CallOpts{
		Pending: false,
		From:    account1,
		Context: context.Background(),
	}
	auth2 := bind.NewKeyedTransactor(key2)
	auth2.GasLimit = uint64(params.GasLimit)
	auth2.GasPrice = big.NewInt(params.GasPrice)
	tx, err := manager.NewChannel(auth1, account2, big.NewInt(35))
	if err != nil {
		log.Printf("Failed to NewChannel: %v,%s,%s", err, auth1.From.String(), account2.String())
		return
	}
	log.Printf("create channel gas %s:%d\n", tx.Hash().String(), tx.Gas())
	ctx := context.Background()
	_, err = bind.WaitMined(ctx, conn, tx)
	if err != nil {
		log.Fatalf("failed to NewChannel when mining :%v", err)
	}
	fmt.Printf("NewChannel complete...\n")
	//step 2 deopsit
	//step 2.1 aprove
	channelAddress, err := manager.GetChannelWith(callAuth1, account2)
	if err != nil {
		log.Fatalf("failed to get channel %s", err)
		return
	}
	channel, _ := contracts.NewNettingChannelContract(channelAddress, conn)
	wg2 := sync.WaitGroup{}
	go func() {
		wg2.Add(1)
		defer wg2.Done()
		tx, err := token.Approve(auth1, channelAddress, big.NewInt(amount))
		if err != nil {
			log.Fatalf("Failed to Approve: %v", err)
		}
		log.Printf("approve gas %s:%d\n", tx.Hash().String(), tx.Gas())
		ctx = context.Background()
		_, err = bind.WaitMined(ctx, conn, tx)
		if err != nil {
			log.Fatalf("failed to Approve when mining :%v", err)
		}
		fmt.Printf("Approve complete...\n")
		tx, err = channel.Deposit(auth1, big.NewInt(amount))
		if err != nil {
			log.Fatalf("Failed to Deposit: %v", err)
		}
		log.Printf("deposit gas %s:%d\n", tx.Hash().String(), tx.Gas())
		ctx = context.Background()
		_, err = bind.WaitMined(ctx, conn, tx)
		if err != nil {
			log.Fatalf("failed to Deposit when mining :%v", err)
		}
		fmt.Printf("Deposit complete...\n")
	}()
	go func() {
		wg2.Add(1)
		defer wg2.Done()
		tx, err := token.Approve(auth2, channelAddress, big.NewInt(amount))
		if err != nil {
			log.Fatalf("Failed to Approve: %v", err)
		}
		ctx = context.Background()
		_, err = bind.WaitMined(ctx, conn, tx)
		if err != nil {
			log.Fatalf("failed to Approve when mining :%v", err)
		}
		fmt.Printf("Approve complete...\n")
		tx, err = channel.Deposit(auth2, big.NewInt(amount))
		if err != nil {
			log.Fatalf("Failed to Deposit: %v", err)
		}
		ctx = context.Background()
		_, err = bind.WaitMined(ctx, conn, tx)
		if err != nil {
			log.Fatalf("failed to Deposit when mining :%v", err)
		}
		fmt.Printf("Deposit complete...\n")
	}()
	time.Sleep(time.Millisecond * 10)
	wg2.Wait()
}
