package ethblockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Event_Log struct {
	client           *RPC_Client
	filterId         string
	contractAddress  string
	formatter        func(*Raw_Event) string
	processor        func(*Event_Log, *Raw_Event, interface{})
	processorContext interface{}
	batchStart       uint64
	batchEnd         uint64
	batchSize        uint64
	anyEvents        bool
}

type Raw_Event struct {
	LogIndex         string   `json:"logIndex"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockNumber      string   `json:"blockNumber"`
	BlockHash        string   `json:"blockHash"`
	Address          string   `json:"address"`
	Data             string   `json:"data"`
	Topics           []string `json:"topics"`
}

// === global state used to detect when we havent seen a block in a while ===
type blockSync struct {
	lastBlockTime int64  // The unix time in seconds when blockNumber was last updated
	blockNumber   string // The last block that was seen, top of the chain
	blockStable   uint64 // The last block that can be read from
}

var global_block_state_lock sync.Mutex
var global_block_state blockSync
var no_recent_blocks int
var block_read_delay int
var block_update_delay int

func update_block(blockNumber uint64) {
	global_block_state_lock.Lock()
	defer global_block_state_lock.Unlock()

	newBlock := fmt.Sprintf("0x%x", blockNumber)
	if global_block_state.blockNumber == "" || newBlock != global_block_state.blockNumber {
		global_block_state.lastBlockTime = time.Now().Unix()
		global_block_state.blockNumber = newBlock
		global_block_state.blockStable = blockNumber - uint64(block_read_delay)
	}
}

func (self *Event_Log) get_stable_block() uint64 {
	delta := time.Now().Unix() - global_block_state.lastBlockTime
	if int(delta) >= block_update_delay {
		if block, err := self.client.Get_block_number(); err != nil {
			glog.Errorf("Error getting current block: %v", err)
		} else {
			update_block(block)
		}
	}

	return global_block_state.blockStable
}

func (self *Event_Log) Get_current_stable_block() string {
	global_block_state_lock.Lock()
	defer global_block_state_lock.Unlock()
	res := fmt.Sprintf("0x%x", global_block_state.blockStable)
	return res
}

// A factory function used to create Event_Log instances
func Event_Log_Factory(httpClientFactory *config.HTTPClientFactory, rpcClient *RPC_Client, contractAddress string) *Event_Log {

	var err error

	if block_read_delay, err = strconv.Atoi(os.Getenv("mtn_soliditycontract_block_read_delay")); err != nil {
		block_read_delay = 0
	}

	if block_update_delay, err = strconv.Atoi(os.Getenv("mtn_soliditycontract_block_update_delay")); err != nil {
		block_update_delay = 10
	}

	rpcc := rpcClient
	if rpcc == nil {
		if rpcc = getRPCClient(httpClientFactory); rpcc == nil {
			return nil
		}
	}

	if len(contractAddress) == 0 {
		glog.Errorf("Input contract address is empty")
		return nil
	} else {
		el := &Event_Log{
			client:          rpcc,
			filterId:        "",
			contractAddress: contractAddress,
		}
		return el
	}
}

func (self *Event_Log) Set_formatter(f func(*Raw_Event) string) {
	self.formatter = f
}

func (self *Event_Log) Set_processor(p func(*Event_Log, *Raw_Event, interface{}), c interface{}) {
	self.processor = p
	self.processorContext = c
}

func (self *Event_Log) Get_Raw_Event_Batch(topics []interface{}, size uint64) ([]Raw_Event, error) {

	self.batchStart = 1
	if bs, err := strconv.Atoi(os.Getenv("bh_event_log_start")); err == nil {
		self.batchStart = uint64(bs)
	}
	self.batchSize = size

	lastBlock := self.get_stable_block()

	if size == 0 {
		self.batchEnd = lastBlock
	} else if lastBlock < (self.batchStart + size - 1) {
		self.batchEnd = lastBlock
	} else {
		self.batchEnd = self.batchStart + size - 1
	}

	if self.batchEnd < self.batchStart {
		self.batchEnd = self.batchStart
	}

	return self.get_raw_events_in_range(topics, self.batchStart, self.batchEnd)
}

func (self *Event_Log) Get_Next_Raw_Event_Batch(topics []interface{}, size uint64) ([]Raw_Event, bool, error) {
	reachedEnd := false

	events := make([]Raw_Event, 0, 10)
	if self.batchEnd == 0 {
		glog.Warningf("For %v previous batch included the latest blocks.", self.contractAddress)
		return events, true, nil
	}

	if err := self.remove_Filter(); err != nil {
		glog.Errorf("For %v could not remove filter: %v", self.contractAddress, err)
	}

	lastBlock := self.get_stable_block()

	start := self.batchEnd + 1
	end := start + size - 1
	if lastBlock <= start {
		return events, true, nil
	} else if size == 0 {
		end = lastBlock
		reachedEnd = true
	} else if lastBlock < (start + size - 1) {
		end = lastBlock
		reachedEnd = true
	}

	if end < start {
		end = start
		reachedEnd = true
	}

	if events, err := self.get_raw_events_in_range(topics, start, end); err != nil {
		return events, true, err
	} else {
		self.batchStart = start
		self.batchSize = size
		self.batchEnd = end
		return events, reachedEnd, nil
	}

}

func (self *Event_Log) get_raw_events_in_range(topics []interface{}, start uint64, end uint64) ([]Raw_Event, error) {
	events := make([]Raw_Event, 0, 10)

	glog.V(3).Infof("EventLog looking for events from %v to %v", start, end)
	if err := self.establish_Filter(start, end, topics); err != nil {
		glog.Errorf("For %v could not establish filter: %v", self.contractAddress, err)
		return events, err
	}

	rpcEventResp := rpcGetEventsResponse{}

	glog.V(5).Infof("Filter %v for %v using client %v ", self.filterId, self.contractAddress, self.client)
	if out, err := self.client.Invoke("eth_getFilterLogs", self.filterId); err != nil {
		glog.Errorf("Error occurred getting events for contract %v, error: %v", self.contractAddress, err)
		return events, errors.New(err.Msg)
	} else if err := json.Unmarshal([]byte(out), &rpcEventResp); err != nil {
		glog.Errorf("Error occurred umarshalling getFilterLogs for %v, response: %v", self.contractAddress, err)
		return events, err
	} else if rpcEventResp.Error.Message != "" {
		glog.Errorf("For %v eth_getFilterLogs returned an error: %v", self.contractAddress, rpcEventResp.Error.Message)
		return events, err
	} else {
		events = rpcEventResp.Result
		if len(events) != 0 {
			self.anyEvents = true
		}
	}

	return events, nil
}

func (self *Event_Log) clear_Filter() {
	self.filterId = ""
}

func (self *Event_Log) establish_Filter(startBlock uint64, endBlock uint64, topics []interface{}) error {
	if self.filterId == "" {
		rpcResp := RPC_Response{}
		params := make(map[string]interface{})
		params["address"] = self.contractAddress
		params["fromBlock"] = fmt.Sprintf("0x%x", startBlock)
		if endBlock != 0 {
			params["toBlock"] = fmt.Sprintf("0x%x", endBlock)
		} else {
			params["toBlock"] = "latest"
		}
		theTopics := topics

		for ix, val := range theTopics {
			switch val.(type) {
			case string:
				v := val.(string)
				if len(v) < 66 && !strings.HasPrefix(v, "0x") {
					v = "0x" + v
				}
				if len(v) < 66 {
					v = v[:2] + strings.Repeat("0", 66-len(v)) + v[2:]
				}
				theTopics[ix] = v
			case nil:
			default:
				return errors.New(fmt.Sprintf("Cannot establish filter, topics must be string or nil, type was %v", reflect.TypeOf(val).String()))
			}
		}
		params["topics"] = theTopics
		glog.V(5).Infof("For %v creating event filter with params: %v", self.contractAddress, params)

		if out, err := self.client.Invoke("eth_newFilter", params); err != nil {
			return errors.New(err.Msg)
		} else if err := json.Unmarshal([]byte(out), &rpcResp); err != nil {
			return err
		} else if rpcResp.Error.Message != "" {
			return errors.New(fmt.Sprintf("Creating event filter returned an error: %v.", rpcResp.Error.Message))
		} else {
			self.filterId = rpcResp.Result.(string)
		}
	}
	return nil
}

func (self *Event_Log) remove_Filter() error {
	if self.filterId != "" {
		rpcResp := RPC_Response{}
		if out, err := self.client.Invoke("eth_uninstallFilter", self.filterId); err != nil {
			return errors.New(err.Msg)
		} else if err := json.Unmarshal([]byte(out), &rpcResp); err != nil {
			return err
		} else if rpcResp.Error.Message != "" {
			return errors.New(fmt.Sprintf("Removing event filter returned an error: %v.", rpcResp.Error.Message))
		} else {
			self.clear_Filter()
			return nil
		}
	} else {
		return nil
	}
}

func getRPCClient(httpClientFactory *config.HTTPClientFactory) *RPC_Client {
	var rpcc *RPC_Client

	if con := RPC_Connection_Factory("", 0, "http://localhost:8545"); con == nil {
		glog.Errorf("RPC Connection not created")
		return nil
	} else if rpcc = RPC_Client_Factory(httpClientFactory, con); rpcc == nil {
		glog.Errorf("RPC Client not created")
		return nil
	}
	return rpcc
}

type rpcGetEventsResponse struct {
	Id      string      `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  []Raw_Event `json:"result"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
