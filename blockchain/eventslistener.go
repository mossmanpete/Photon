package blockchain

import (
	"strings"

	"errors"
	"math/big"

	"bytes"
	"fmt"

	"encoding/hex"

	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc"
	"github.com/SmartMeshFoundation/SmartRaiden/params"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

/*
query and subscribe may get other contract's event which has the same name and arguments.
*/
var eventTokenAddedId common.Hash
var eventChannelNewId common.Hash
var eventChannelDeletedId common.Hash
var eventChannelNewBalanceId common.Hash
var eventChannelClosedId common.Hash
var eventTransferUpdatedId common.Hash
var eventChannelSettledId common.Hash
var eventChannelSecretRevealedId common.Hash
var eventAddressRegisteredId common.Hash
var errEventNotMatch = errors.New("")

type Eventer interface {
	Name() string
}
type Event struct {
	EventName       string
	BlockNumber     int64
	TxIndex         uint
	TxHash          common.Hash
	ContractAddress common.Address
}

func (this *Event) Name() string {
	return this.EventName
}
func initEventWithLog(el *types.Log, e *Event) {
	e.BlockNumber = int64(el.BlockNumber)
	e.TxIndex = el.TxIndex
	e.TxHash = el.TxHash
	e.ContractAddress = el.Address
}

type EventTokenAdded struct {
	Event
	TokenAddress          common.Address
	ChannelManagerAddress common.Address
}

func debugPrintLog(l *types.Log) {
	w := new(bytes.Buffer)
	fmt.Fprintf(w, "{\nblocknumber=%d,txIndex=%d,Index=%d,Address=%s\n",
		l.BlockNumber, l.TxIndex, l.Index, utils.APex(l.Address))
	for i, t := range l.Topics {
		fmt.Fprintf(w, "topics[%d]=%s\n", i, t.String())
	}
	fmt.Fprintf(w, "data:\n%s\n}", hex.Dump(l.Data))
	log.Trace(string(w.Bytes()))
}
func NewEventTokenAdded(el *types.Log) (e *EventTokenAdded, err error) {
	if len(el.Data) < 64 {
		err = errEventNotMatch
		return
	}
	e = &EventTokenAdded{}
	e.EventName = params.NameTokenAdded
	if eventTokenAddedId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.RegistryABI))
		if err != nil {
			return nil, err
		}
		eventTokenAddedId = parsed.Events[e.EventName].Id()
	}
	if eventTokenAddedId != el.Topics[0] {
		log.Crit("NewEventTokenAdded with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.TokenAddress = common.BytesToAddress(el.Data[12:32])          // the first 32byte is tokenaddress
	e.ChannelManagerAddress = common.BytesToAddress(el.Data[44:64]) //the second 32byte is channelManagerAddress
	return
}

type EventChannelNew struct {
	Event
	NettingChannelAddress common.Address
	Participant1          common.Address
	Participant2          common.Address
	SettleTimeout         int
}

func NewEventEventChannelNew(el *types.Log) (e *EventChannelNew, err error) {
	if len(el.Data) < 128 {
		err = errEventNotMatch
		return
	}
	e = &EventChannelNew{}
	e.EventName = params.NameChannelNew
	if eventChannelNewId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.ChannelManagerContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelNewId = parsed.Events[e.EventName].Id()
	}
	if eventChannelNewId != el.Topics[0] {
		log.Crit("NewEventEventChannelNew with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.NettingChannelAddress = common.BytesToAddress(el.Data[12:32]) //the first 32byte is tokenaddress
	e.Participant1 = common.BytesToAddress(el.Data[44:64])          //the second 32byte is channelManagerAddress
	e.Participant2 = common.BytesToAddress(el.Data[76:96])
	t := new(big.Int)
	t.SetBytes(el.Data[96:128])
	e.SettleTimeout = int(t.Int64())
	return
}

type EventChannelDeleted struct {
	Event
	CallerAddress common.Address
	Partener      common.Address
}

func NewEventChannelDeleted(el *types.Log) (e *EventChannelDeleted, err error) {
	if len(el.Data) < 64 {
		err = errEventNotMatch
		return
	}
	e = &EventChannelDeleted{}
	e.EventName = params.NameChannelDeleted
	if eventChannelDeletedId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.ChannelManagerContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelDeletedId = parsed.Events[e.EventName].Id()
	}
	if eventChannelDeletedId != el.Topics[0] {
		log.Crit("NewEventEventChannelNew with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.CallerAddress = common.BytesToAddress(el.Data[12:32]) //the first 32byte is tokenaddress
	e.Partener = common.BytesToAddress(el.Data[44:64])      //the second 32byte is channelManagerAddress
	return
}

type EventChannelNewBalance struct {
	Event
	TokenAddress       common.Address
	ParticipantAddress common.Address
	Balance            *big.Int
}

func NewEventChannelNewBalance(el *types.Log) (e *EventChannelNewBalance, err error) {
	if len(el.Data) < 96 {
		err = errEventNotMatch
		return
	}
	e = &EventChannelNewBalance{}
	e.EventName = params.NameChannelNewBalance
	if eventChannelNewBalanceId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.NettingChannelContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelNewBalanceId = parsed.Events[e.EventName].Id()
	}
	if eventChannelNewBalanceId != el.Topics[0] {
		log.Crit("NewEventChannelNewBalance with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.TokenAddress = common.BytesToAddress(el.Data[12:32])       //the first 32byte is tokenaddress
	e.ParticipantAddress = common.BytesToAddress(el.Data[44:64]) //the second 32byte is channelManagerAddress
	t := new(big.Int)
	t.SetBytes(el.Data[64:96])
	e.Balance = t
	return
}

//event ChannelClosed(address closing_address, uint block_number);
type EventChannelClosed struct {
	Event
	ClosingAddress common.Address
}

func NewEventChannelClosed(el *types.Log) (e *EventChannelClosed, err error) {
	if len(el.Data) < 32 {
		err = errEventNotMatch
		return
	}
	e = &EventChannelClosed{}
	e.EventName = params.NameChannelClosed
	if eventChannelClosedId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.NettingChannelContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelClosedId = parsed.Events[e.EventName].Id()
	}
	if eventChannelClosedId != el.Topics[0] {
		log.Crit("NewEventChannelClosed with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.ClosingAddress = common.BytesToAddress(el.Data[12:32]) //the first 32byte is tokenaddress
	return
}

//
//event TransferUpdated(address node_address, uint block_number);

type EventTransferUpdated struct {
	Event
	NodeAddress common.Address
}

func NewEventTransferUpdated(el *types.Log) (e *EventTransferUpdated, err error) {
	if len(el.Data) < 32 {
		err = errEventNotMatch
		return
	}
	e = &EventTransferUpdated{}
	e.EventName = params.NameTransferUpdated
	if eventTransferUpdatedId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.NettingChannelContractABI))
		if err != nil {
			return nil, err
		}
		eventTransferUpdatedId = parsed.Events[e.EventName].Id()
	}
	if eventTransferUpdatedId != el.Topics[0] {
		log.Crit("NewEventTransferUpdatedd with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.NodeAddress = common.BytesToAddress(el.Data[12:32]) //the first 32byte is tokenaddress
	return
}

//event ChannelSettled(uint block_number);

type EventChannelSettled struct {
	Event
}

func NewEventChannelSettled(el *types.Log) (e *EventChannelSettled, err error) {
	e = &EventChannelSettled{}
	e.EventName = params.NameChannelSettled
	if eventChannelSettledId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.NettingChannelContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelSettledId = parsed.Events[e.EventName].Id()
	}
	if eventChannelSettledId != el.Topics[0] {
		log.Crit("NewEventChannelSettledd with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	return
}

//event ChannelSecretRevealed(bytes32 secret, address receiver_address);
type EventChannelSecretRevealed struct {
	Event
	Secret          common.Hash
	ReceiverAddress common.Address
}

func NewEventChannelSecretRevealed(el *types.Log) (e *EventChannelSecretRevealed, err error) {
	if len(el.Data) < 64 {
		err = errEventNotMatch
		return
	}
	e = &EventChannelSecretRevealed{}
	e.EventName = params.NameChannelSecretRevealed
	if eventChannelSecretRevealedId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.NettingChannelContractABI))
		if err != nil {
			return nil, err
		}
		eventChannelSecretRevealedId = parsed.Events[e.EventName].Id()
	}
	if eventChannelSecretRevealedId != el.Topics[0] {
		log.Crit("NewEventChannelSecretRevealedd with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.Secret = common.BytesToHash(el.Data[:32]) //the first 32byte is secret,the second is address
	e.ReceiverAddress = common.BytesToAddress(el.Data[44:64])
	return
}

// event AddressRegistered(address indexed eth_address, string socket);

type EventAddressRegistered struct {
	Event
	EthAddress common.Address
	Socket     string
}

func NewEventAddressRegistered(el *types.Log) (e *EventAddressRegistered, err error) {
	if len(el.Data) < 64 {
		err = errEventNotMatch
		return
	}
	e = &EventAddressRegistered{}
	e.EventName = params.NameAddressRegistered
	if eventAddressRegisteredId == utils.EmptyHash {
		//no error test,the abi is generated by abigen
		parsed, err := abi.JSON(strings.NewReader(rpc.EndpointRegistryABI))
		if err != nil {
			return nil, err
		}
		eventAddressRegisteredId = parsed.Events[e.EventName].Id()
	}
	if eventAddressRegisteredId != el.Topics[0] {
		log.Crit("NewEventAddressRegisteredd with unknown log: ", el)
	}
	initEventWithLog(el, &e.Event)
	e.EthAddress = common.BytesToAddress(el.Topics[1][12:32]) //
	/* Data todo why is  first 32bytes empty?
		Data: ([]uint8) (len=96 cap=96) {
	            00000000  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
	            00000010  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 20  |............... |
	            00000020  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
	            00000030  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 12  |................|
	            00000040  31 37 32 2e 33 31 2e 37  30 2e 32 38 3a 34 30 30  |172.31.70.28:400|
	            00000050  30 31 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |01..............|
	        },
	*/
	//the first 32 bytes in the data are empty,what does it mean?
	t := new(big.Int)
	t.SetBytes(el.Data[32:64])
	if len(el.Data) < 64+int(t.Int64()) {
		err = errEventNotMatch
		return
	}
	e.Socket = string(el.Data[64 : 64+int(t.Int64())])
	return
}
