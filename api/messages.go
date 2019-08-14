package api

import (
	"github.com/open-horizon/anax/i18n"
)

// messages for event logs
const (
	// from api_node.go
	EL_API_ERR_PARSING_INPUT_FOR_NODE_REG          = "Error parsing input for node configuration/registration. Input body couldn't be deserialized to node object: %v, error: %v"
	EL_API_ERR_PARSING_INPUT_FOR_NODE_UNREG        = "Error parsing input for node configuration/registration. Input body couldn't be deserialized to configstate object: %v, error: %v"
	EL_API_ERR_PARSING_INPUT_FOR_NODE_UPDATE       = "Error parsing input for node update. Input body couldn't be deserialized to node object: %v, error: %v"
	EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY       = "Error parsing input for node policy. Input body could not be deserialized as a policy object: %v, error: %v"
	EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY_PATCH = "Error parsing input for node policy patch. Input body could not be deserialized into a Constraint Expression or Property List: %v, error: %v"
	EL_API_ERR_POLICY_PATCH_INPUT_PROPERTY_ERROR   = "Error parsing input for node policy patch. Input body did not contain a Constraint Expression or Property List: %v, error: %v"
	EL_API_ERR_PARSING_INPUT_FOR_NODE_UI           = "Error parsing input for node user input. Input body could not be deserialized as a UserInput object: %v, error: %v"

	EL_API_ERR_IN_NODE_REG            = "Error in node configuration/registration for node %v. %v"
	EL_API_ERR_IN_NODE_UPDATE         = "Error in updating node %v. %v"
	EL_API_ERR_IN_NODE_UNREG          = "Error in node unregistration. %v"
	EL_API_ERR_IN_VERIFY_EXCH_VERSION = "Error verifiying exchange version. error: %v"
	EL_API_ERR_IN_NODE_POLICY_CREATE  = "Error in creating or replacing node policy. %v"
	EL_API_ERR_IN_NODE_POLICY_PATCH   = "Error in patching node policy. %v"
	EL_API_ERR_IN_NODE_POLICY_DEL     = "Error in deleting node policy. %v"
	EL_API_ERR_IN_NODE_UI_UPDATE      = "Error in updating node user input. %v"
	EL_API_ERR_IN_NODE_UI_PATCH       = "Error in patching node user input. %v"
	EL_API_ERR_IN_NODE_UI_DEL         = "Error in deleting node userinput. %v"

	// from path_node.go
	EL_API_START_NODE_REG       = "Start node configuration/registration for node %v."
	EL_API_START_NODE_UPDATE    = "Start updating node %v."
	EL_API_COMPLETE_NODE_UPDATE = "Complete node update for %v."
	EL_API_START_NODE_UNREG     = "Start node unregistration."
	EL_API_COMPLETE_NODE_UNREG  = "Node unregistration complete for node %v."

	EL_API_ERR_NODE_UNREG_NOT_FOUND             = "Error unregistring the node. The node is not found from the database."
	EL_API_ERR_NODE_UNREG_NOT_IN_STATE          = "Error unregistring the node. The node must be in 'configured' or 'configuring' state in order to unconfigure it."
	EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_RN    = "Input error for node unregistration. %v is an incorrect value for removeNode"
	EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_DC    = "Input error for node unregistration. %v is an incorrect value for deepClean"
	EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_BLOCK = "Input error for node unregistration. %v is an incorrect value for block"

	EL_API_ERR_READ_NODE_FROM_DB    = "Unable to read node object from database, error %v"
	EL_API_ERR_SAVE_NODE_CONF_TO_DB = "Error saving new node config state (unconfiguring) in the database: %v"

	// from path_node_configstate.go
	EL_API_ERR_NODE_CONF_NOT_FOUND    = "Error in node configuration. The node is not found from the database."
	EL_API_ERR_NODE_CONF_WRONG_STATE  = "Error in node configuration. The node must be in 'configured' or 'configuring' state in order to change the state to %v."
	EL_API_UNSUP_NODE_STATE_TRANS     = "Node state transition from '%v' to '%v' is not supported."
	EL_API_FAIL_GET_UI_FROM_DB        = "Failed get user input from local db. %v"
	EL_API_FAIL_FIND_SVC_PREF_FROM_UI = "Failed to find preferences for service %v/%v from the local user input, error: %v"
	EL_API_ERR_SAVE_NODE_CONFSTATE    = "Error saving new node config state to database: %v"
	EL_API_COMPLETE_NODE_REG          = "Complete node configuration/registration for node %v."
	EL_API_ERR_SVC_CONF               = "Error in service configuration for %v. %v"
	EL_API_ERR_GET_SREFS_FOR_PATTERN  = "Error getting service references for pattern %v. %v"

	// from path_node_policy.go
	EL_API_NEW_NODE_POL     = "New node policy: %v"
	EL_API_NODE_POL_DELETED = "Deleted node policy"

	// from path_node_userinput.go
	EL_API_NEW_NODE_UI         = "New node user input: %v"
	EL_API_NO_NODE_UI_TO_DEL   = "No node user input to detele"
	EL_API_DELETED_ALL_NODE_UI = "Deleted all node user input"

	// from path_service_config.go
	EL_API_START_SVC_CONFIG           = "Start service configuration with user input for %v/%v."
	EL_API_START_SVC_AUTO_CONFIG      = "Start service auto configuration for %v/%v."
	EL_API_COMPLETE_SVC_CONFIG        = "Complete service configuration for %v/%v."
	EL_API_COMPLETE_SVC_AUTO_CONFIG   = "Complete service auto configuration for %v/%v."
	EL_API_ERR_MISS_VAR_IN_SVC_CONFIG = "Variable %v is missing in the service configuration for %v/%v. It may cause agreement not formed if the business policy does not contain the setting for the missing variable."

	// from api_service.go
	EL_API_ERR_CONFIG_SVC                  = "Error configuring service %v. %v"
	EL_API_ERR_CHANGE_SVC_CONFIGSTATE      = "Error changing service configstate %v, error %v"
	EL_API_START_CHANGE_SVC_CONFIGSTATE    = "Start changing service configuration state to %v for %v for the node."
	EL_API_COMPLETE_CHANGE_SVC_CONFIGSTATE = "Complete changing service configuration state to %v for %v for the node."
)

// This is does nothing useful at run time.
// This code is only used in compileing time to make the eventlog messages gets into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	// from api_node.go
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_REG)
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_UNREG)
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_UPDATE)
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY)
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_POLICY_PATCH)
	msgPrinter.Sprintf(EL_API_ERR_POLICY_PATCH_INPUT_PROPERTY_ERROR)
	msgPrinter.Sprintf(EL_API_ERR_PARSING_INPUT_FOR_NODE_UI)

	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_REG)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_UPDATE)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_UNREG)
	msgPrinter.Sprintf(EL_API_ERR_IN_VERIFY_EXCH_VERSION)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_POLICY_CREATE)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_POLICY_PATCH)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_POLICY_DEL)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_UI_UPDATE)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_UI_PATCH)
	msgPrinter.Sprintf(EL_API_ERR_IN_NODE_UI_DEL)

	// from path_node.go
	msgPrinter.Sprintf(EL_API_START_NODE_REG)
	msgPrinter.Sprintf(EL_API_START_NODE_UPDATE)
	msgPrinter.Sprintf(EL_API_COMPLETE_NODE_UPDATE)
	msgPrinter.Sprintf(EL_API_START_NODE_UNREG)
	msgPrinter.Sprintf(EL_API_COMPLETE_NODE_UNREG)

	msgPrinter.Sprintf(EL_API_ERR_NODE_UNREG_NOT_FOUND)
	msgPrinter.Sprintf(EL_API_ERR_NODE_UNREG_NOT_IN_STATE)
	msgPrinter.Sprintf(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_RN)
	msgPrinter.Sprintf(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_DC)
	msgPrinter.Sprintf(EL_API_ERR_NODE_UNREG_WRONG_VALUE_FOR_BLOCK)

	msgPrinter.Sprintf(EL_API_ERR_READ_NODE_FROM_DB)
	msgPrinter.Sprintf(EL_API_ERR_SAVE_NODE_CONF_TO_DB)

	// from path_node_configstate.go
	msgPrinter.Sprintf(EL_API_ERR_NODE_CONF_NOT_FOUND)
	msgPrinter.Sprintf(EL_API_ERR_NODE_CONF_WRONG_STATE)
	msgPrinter.Sprintf(EL_API_UNSUP_NODE_STATE_TRANS)
	msgPrinter.Sprintf(EL_API_FAIL_GET_UI_FROM_DB)
	msgPrinter.Sprintf(EL_API_FAIL_FIND_SVC_PREF_FROM_UI)
	msgPrinter.Sprintf(EL_API_ERR_SAVE_NODE_CONFSTATE)
	msgPrinter.Sprintf(EL_API_COMPLETE_NODE_REG)
	msgPrinter.Sprintf(EL_API_ERR_SVC_CONF)
	msgPrinter.Sprintf(EL_API_ERR_GET_SREFS_FOR_PATTERN)

	// from path_node_policy.go
	msgPrinter.Sprintf(EL_API_NEW_NODE_POL)
	msgPrinter.Sprintf(EL_API_NODE_POL_DELETED)

	// from path_node_userinput.go
	msgPrinter.Sprintf(EL_API_NEW_NODE_UI)
	msgPrinter.Sprintf(EL_API_NO_NODE_UI_TO_DEL)
	msgPrinter.Sprintf(EL_API_DELETED_ALL_NODE_UI)

	// from path_service_config.go
	msgPrinter.Sprintf(EL_API_START_SVC_CONFIG)
	msgPrinter.Sprintf(EL_API_START_SVC_AUTO_CONFIG)
	msgPrinter.Sprintf(EL_API_COMPLETE_SVC_CONFIG)
	msgPrinter.Sprintf(EL_API_COMPLETE_SVC_AUTO_CONFIG)
	msgPrinter.Sprintf(EL_API_ERR_MISS_VAR_IN_SVC_CONFIG)

	// from api_service.go
	msgPrinter.Sprintf(EL_API_ERR_CONFIG_SVC)
	msgPrinter.Sprintf(EL_API_ERR_CHANGE_SVC_CONFIGSTATE)
	msgPrinter.Sprintf(EL_API_START_CHANGE_SVC_CONFIGSTATE)
	msgPrinter.Sprintf(EL_API_COMPLETE_CHANGE_SVC_CONFIGSTATE)
}
