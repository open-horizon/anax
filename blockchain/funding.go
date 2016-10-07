package blockchain

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"repo.hovitos.engineering/MTN/anax/config"
	"strings"
)

// similar to what's in the whisper pkg, consider consolidating
type RPCMsg struct {
	JsonRPC string `json:"jsonrpc"`
	Id      int    `json:"id"`
}

type RPCRequest struct {
	RPCMsg
	Params []string `json:"params"`
	Method string   `json:"method"`
}

type RPCResponse struct {
	RPCMsg
	Result string `json:"result"`
}

func NewRPCRequest(method string, params []string) *RPCRequest {
	return &RPCRequest{
		RPCMsg: RPCMsg{
			JsonRPC: "2.0",
			Id:      2,
		},
		Method: method,
		Params: params,
	}
}

func AccountFunded(config *config.Config) (bool, error) {

	if account, err := AccountId(); err != nil {
		return false, err
	} else {
		params := make([]string, 0)
		params = append(params, fmt.Sprintf("0x%v", account))
		params = append(params, "latest")

		msg := NewRPCRequest("eth_getBalance", params)

		if req, err := json.Marshal(msg); err != nil {
			return false, err
		} else {

			if response, err := http.Post(config.GethURL, "application/json", strings.NewReader(string(req[:]))); err != nil {
				return false, err
			} else if response.StatusCode != 200 {
				return false, fmt.Errorf("Got non-200 response code from Post to determine if account is funded: %v", response.StatusCode)
			} else {
				defer response.Body.Close()

				if content, err := ioutil.ReadAll(response.Body); err != nil {
					return false, err
				} else {

					var response RPCResponse
					if err := json.Unmarshal(content, &response); err != nil {
						return false, err
					} else {
						dec := new(big.Int)

						if dec, success := dec.SetString(strings.TrimPrefix(response.Result, "0x"), 16); !success {
							return false, nil
						} else {
							zero := big.NewInt(0)
							return (dec.Cmp(zero) > 0), nil
						}
					}
				}
			}
		}
	}
}
