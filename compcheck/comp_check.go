package compcheck

import (
	"fmt"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/i18n"
	"golang.org/x/text/message"
)

// **** Note: All the errors returned by the functions in this component are of type *CompCheckError,
// You can cast it by err.(*compcheck.CompCheckError) if you want to get the error code.

// error code for compatibility check error: CompCheckError
const (
	COMPCHECK_INPUT_ERROR      = 10
	COMPCHECK_VALIDATION_ERROR = 11
	COMPCHECK_CONVERTION_ERROR = 12
	COMPCHECK_MERGING_ERROR    = 13
	COMPCHECK_EXCHANGE_ERROR   = 14
	COMPCHECK_GENERAL_ERROR    = 15
)

// Error for policy compatibility check
type CompCheckError struct {
	Err     string `json:"error"`
	ErrCode int    `json:"error_code,omitempty"`
}

func (e *CompCheckError) Error() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("%v", e.Err)
	}
}

func (e *CompCheckError) String() string {
	if e == nil {
		return ""
	} else {
		return fmt.Sprintf("ErrCode: %v, Error: %v", e.ErrCode, e.Err)
	}
}

func NewCompCheckError(err error, errCode int) *CompCheckError {
	return &CompCheckError{
		Err:     err.Error(),
		ErrCode: errCode,
	}
}

// Get the exchange device
func GetExchangeNode(getDeviceHandler exchange.DeviceHandler, nodeId string, msgPrinter *message.Printer) (*exchange.Device, error) {
	// get default message printer if nil
	if msgPrinter == nil {
		msgPrinter = i18n.GetMessagePrinter()
	}

	// check input
	if nodeId == "" {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("The given node id is empty.")), COMPCHECK_INPUT_ERROR)
	} else {
		nodeOrg := exchange.GetOrg(nodeId)
		if nodeOrg == "" {
			return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Organization is not specified in the given node id: %v.", nodeId)), COMPCHECK_INPUT_ERROR)
		}
	}

	if node, err := getDeviceHandler(nodeId, ""); err != nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("Error getting node %v from the exchange. %v", nodeId, err)), COMPCHECK_EXCHANGE_ERROR)
	} else if node == nil {
		return nil, NewCompCheckError(fmt.Errorf(msgPrinter.Sprintf("No node found for this node id %v.", nodeId)), COMPCHECK_INPUT_ERROR)
	} else {
		return node, nil
	}
}
