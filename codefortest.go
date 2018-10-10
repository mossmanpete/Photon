package smartraiden

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path"

	"time"

	"encoding/hex"

	"sync"

	"github.com/SmartMeshFoundation/SmartRaiden/accounts"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/models"
	"github.com/SmartMeshFoundation/SmartRaiden/network"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc/fee"
	"github.com/SmartMeshFoundation/SmartRaiden/notify"
	"github.com/SmartMeshFoundation/SmartRaiden/params"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

//reinit this variable before test case start
var curAccountIndex = 0

func reinit() {
	curAccountIndex = 0
}
func newTestRaiden() *RaidenService {
	return newTestRaidenWithPolicy(&NoFeePolicy{})
}

func newTestRaidenWithPolicy(feePolicy fee.Charger) *RaidenService {
	config := params.DefaultConfig
	config.DataDir = os.Getenv("DATADIR")
	if config.DataDir == "" {
		config.DataDir = path.Join(os.TempDir(), utils.RandomString(10))
	}
	config.DataBasePath = path.Join(config.DataDir, "log.db")
	db, err := models.OpenDb(config.DataBasePath)
	if err != nil {
		panic(err)
	}
	bcs := newTestBlockChainService(db)
	notifyHandler := notify.NewNotifyHandler()
	transport := network.MakeTestMixTransport(utils.APex2(bcs.NodeAddress), bcs.PrivKey)
	config.MyAddress = bcs.NodeAddress
	config.PrivateKey = bcs.PrivKey
	log.Info(fmt.Sprintf("DataDir=%s", config.DataDir))
	config.RevealTimeout = 10
	config.SettleTimeout = 600
	config.PrivateKeyHex = hex.EncodeToString(crypto.FromECDSA(config.PrivateKey))
	err = os.MkdirAll(config.DataDir, os.ModePerm)
	if err != nil {
		log.Error(err.Error())
	}
	config.NetworkMode = params.MixUDPXMPP
	rd, err := NewRaidenService(bcs, bcs.PrivKey, transport, &config, notifyHandler, db)
	if err != nil {
		log.Error(err.Error())
	}
	rd.SetFeePolicy(feePolicy)
	return rd
}
func newTestRaidenAPI() *RaidenAPI {
	api := NewRaidenAPI(newTestRaiden())
	err := api.Raiden.Start()
	if err != nil {
		panic(fmt.Sprintf("raiden start err %s", err))
	}
	return api
}

//maker sure these accounts are valid, and  engouh eths for test
func testGetnextValidAccount() (*ecdsa.PrivateKey, common.Address) {
	am := accounts.NewAccountManager("testdata/keystore")
	privkeybin, err := am.GetPrivateKey(am.Accounts[curAccountIndex].Address, "123")
	if err != nil {
		log.Error(fmt.Sprintf("testGetnextValidAccount err: %s", err))
		panic("")
	}
	curAccountIndex++
	privkey, err := crypto.ToECDSA(privkeybin)
	if err != nil {
		log.Error(fmt.Sprintf("to privkey err %s", err))
		panic("")
	}
	return privkey, crypto.PubkeyToAddress(privkey.PublicKey)
}
func newTestBlockChainService(db *models.ModelDB) *rpc.BlockChainService {
	privkey, addr := testGetnextValidAccount()
	log.Trace(fmt.Sprintf("privkey=%s,addr=%s", privkey, addr.String()))
	config := &params.Config{
		EthRPCEndPoint:  rpc.TestRPCEndpoint,
		PrivateKey:      privkey,
		RegistryAddress: rpc.PrivateRopstenRegistryAddress,
	}
	bcs, err := rpc.NewBlockChainService(config, db)
	if err != nil {
		log.Error(err.Error())
	}
	return bcs
}

func makeTestRaidens() (r1, r2, r3 *RaidenService) {
	r1 = newTestRaiden()
	r2 = newTestRaiden()
	r3 = newTestRaiden()
	go func() {
		/*#nosec*/
		r1.Start()
	}()
	go func() {
		/*#nosec*/
		r2.Start()
	}()
	go func() {
		/*#nosec*/
		r3.Start()
	}()
	time.Sleep(time.Second * 3)
	return
}
func newTestRaidenAPIQuick() *RaidenAPI {
	api := NewRaidenAPI(newTestRaiden())
	//go func() {
	//	/*#nosec*/
	//	api.Raiden.Start()
	//}()
	return api
}

func makeTestRaidenAPIs() (rA, rB, rC, rD *RaidenAPI) {
	rA = newTestRaidenAPIQuick()
	rB = newTestRaidenAPIQuick()
	rC = newTestRaidenAPIQuick()
	rD = newTestRaidenAPIQuick()
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() {
		/*#nosec*/
		rA.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rB.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rC.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rD.Raiden.Start()
		wg.Done()
	}()
	wg.Wait()
	return
}

func makeTestRaidenAPIArrays(datadirs ...string) (apis []*RaidenAPI) {
	if datadirs == nil || len(datadirs) == 0 {
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(len(datadirs))
	for _, datadir := range datadirs {
		// #nosec
		os.Setenv("DATADIR", datadir)
		api := newTestRaidenAPIQuick()
		go func() {
			/*#nosec*/
			api.Raiden.Start()
			wg.Done()
		}()
		apis = append(apis, api)
	}
	wg.Wait()
	return
}

func makeTestRaidenAPIsWithFee(policy fee.Charger) (rA, rB, rC, rD *RaidenAPI) {
	rA = NewRaidenAPI(newTestRaidenWithPolicy(policy))
	rB = NewRaidenAPI(newTestRaidenWithPolicy(policy))
	rC = NewRaidenAPI(newTestRaidenWithPolicy(policy))
	rD = NewRaidenAPI(newTestRaidenWithPolicy(policy))
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() {
		/*#nosec*/
		rA.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rB.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rC.Raiden.Start()
		wg.Done()
	}()
	go func() {
		/*#nosec*/
		rD.Raiden.Start()
		wg.Done()
	}()
	wg.Wait()
	return
}
