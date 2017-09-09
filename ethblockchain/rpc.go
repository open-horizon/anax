package ethblockchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"
)

type RPC_Client struct {
	connection *RPC_Connection
	body       map[string]interface{}
	httpClient *http.Client
}

func RPC_Client_Factory(connection *RPC_Connection) *RPC_Client {
	if connection == nil {
		glog.Errorf("Input connection is nil")
		return nil
	} else {
		rpcc := &RPC_Client{
			connection: connection,
			body:       make(map[string]interface{}),
		}
		rpcc.body["jsonrpc"] = "2.0"
		rpcc.body["id"] = "1"

		var rpc_timeout time.Duration
		if rpc_t, err := strconv.Atoi(os.Getenv("bh_rpc_timeout")); err != nil || rpc_t == 0 {
			rpc_timeout = time.Duration(config.HTTPDEFAULTTIMEOUT * time.Millisecond)
		} else {
			rpc_timeout = time.Duration(time.Duration(rpc_t) * time.Second)
		}

		rpcc.httpClient = &http.Client{
			Timeout: time.Duration(rpc_timeout),
		}
		return rpcc
	}
}

func (self *RPC_Client) Get_connection() *RPC_Connection {
	return self.connection
}

func (self *RPC_Client) Get_block_number() (uint64, error) {

	if out, err := self.Invoke("eth_blockNumber", nil); err != nil {
		return 0, errors.New(err.Msg)
	} else if rpcResp, err := self.decodeResponse([]byte(out)); err != nil {
		return 0, err
	} else if block, err := strconv.ParseUint(rpcResp.Result.(string)[2:], 16, 64); err != nil {
		return 0, err
	} else {
		return block, nil
	}

	return 0, nil
}

func (self *RPC_Client) Get_balance(address string) (*big.Int, error) {

	bal := big.NewInt(0)
	p := make([]interface{}, 0, 2)
	p = append(p, address)
	p = append(p, "latest")

	if out, err := self.Invoke("eth_getBalance", p); err != nil {
		return bal, errors.New(err.Msg)
	} else if rpcResp, err := self.decodeResponse([]byte(out)); err != nil {
		return bal, err
	} else {
		bal_hex_str := rpcResp.Result.(string)
		// the math/big library doesn't like leading "0x" on hex strings
		bal.SetString(bal_hex_str[2:], 16)
	}

	return bal, nil
}

func (self *RPC_Client) Get_first_account() (string, error) {

	if out, err := self.Invoke("eth_accounts", nil); err != nil {
		return "", errors.New(err.Msg)
	} else if rpcResp, err := self.decodeResponse([]byte(out)); err != nil {
		return "", err
	} else {
		var resp []interface{}
		resp = rpcResp.Result.([]interface{})
		if len(resp) == 0 {
			return "", errors.New("No accounts returned")
		} else {
			acct := resp[0].(string)
			return acct, nil
		}
	}
}

func (self *RPC_Client) Invoke(method string, params interface{}) (string, *RPCError) {

	out := ""
	var err *RPCError

	if len(method) == 0 {
		err = &RPCError{fmt.Sprintf("RPC method name must be non-empty")}
		glog.Errorf("Error: %v", err.Msg)
		return out, err
	}

	self.body["method"] = method

	switch params.(type) {
	case []interface{}:
		self.body["params"] = params
	default:
		the_params := make([]interface{}, 0, 5)
		self.body["params"] = append(the_params, params)
	}

	glog.V(5).Infof("Invoking %v with %v", method, self.body)

	if jsonBytes, e := json.Marshal(self.body); e != nil {
		err = &RPCError{fmt.Sprintf("RPC invocation of %v failed creating JSON body %v, error: %v", method, self.body, e.Error())}
	} else if req, e := http.NewRequest("POST", self.connection.Get_fullURL(), bytes.NewBuffer(jsonBytes)); e != nil {
		err = &RPCError{fmt.Sprintf("RPC invocation of %v failed creating http request, error: %v", method, e.Error())}
	} else {
		req.Close = true // work around to ensure that Go doesn't get connections confused. Supposed to be fixed in Go 1.6.
		if resp, e := self.httpClient.Do(req); e != nil {
			err = &RPCError{fmt.Sprintf("RPC http invocation of %v with %v returned error: %v", method, self.body, e.Error())}
		} else {
			defer resp.Body.Close()
			if outBytes, e := ioutil.ReadAll(resp.Body); e != nil {
				err = &RPCError{fmt.Sprintf("RPC invocation of %v failed reading response message, error: %v", method, outBytes, e.Error())}
			} else {
				out = string(outBytes)
				glog.V(5).Infof("Response to %v is %v", self.body, out)
			}
		}
	}

	if err != nil {
		glog.Errorf("Error: %v", err.Msg)
	}

	return out, err
}

func (self *RPC_Client) decodeResponse(out []byte) (*RPC_Response, error) {
	rpcResp := RPC_Response{}
	if err := json.Unmarshal(out, &rpcResp); err != nil {
		return nil, err
	} else if rpcResp.Error.Message != "" {
		return nil, errors.New(rpcResp.Error.Message)
	} else {
		return &rpcResp, nil
	}
}

type RPC_Response struct {
	Id      string      `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type RPCError struct {
	Msg string
}
