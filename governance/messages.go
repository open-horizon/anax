package governance

import (
	"github.com/open-horizon/anax/i18n"
)

// messages for event logs
const (
	//db
	EL_GOV_ERR_RETRIEVE_AG_FROM_DB            = "Error retrieving agreement %v from database, error %v"
	EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_MNM    = "Unable to retrieve agreement %v from database for MeteringNotification message, error %v"
	EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_CANM   = "Unable to retrieve agreement %v from database for Cancel message, error %v"
	EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_RAM    = "Unable to retrieve agreement %v from database for ReplyAck message, error %v"
	EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_DRM    = "Unable to retrieve agreement %v from database for DataReceived message, error %v"
	EL_GOV_ERR_RETRIEVE_UNARCHIVED_AG_FROM_DB = "Unable to retrieve unarchived agreements from database. %v"
	EL_GOV_ERR_MARK_AG_TERMINATED_IN_DB       = "Error marking agreement %v terminated in database: %v."
	EL_GOV_ERR_RETRIEVE_SDEFS_FROM_DB         = "Error getting service definitions %v from db. %v"
	EL_GOV_ERR_RETRIEVE_SINSTS_VER_FROM_DB    = "Error retrieving all service instances from database for %v/%v version %v key %v. %v"
	EL_GOV_ERR_RETRIEVE_SINSTS_FOR_FROM_DB    = "Error retrieving all service instances from database for %v. %v"
	EL_GOV_ERR_RETRIEVE_SINSTS_FROM_DB        = "Error retrieving all service instances from database, error: %v"
	EL_GOV_ERR_RETRIEVE_SINST_FROM_DB         = "Error getting service instance %v from db. %v"
	EL_GOV_ERR_RETRIEVE_MATCH_AGS_FROM_DB     = "Error retrieving matching agreements from database for workloads %v. Error: %v"
	EL_GOV_ERR_SAVE_NODE_CONFIGSTATE_TO_DB    = "Error perisisting node config state in database to %v. Error: %v"
	EL_GOV_ERR_RETRIEVE_DEVICE_FROM_DB        = "Error retrieving device from database. Error: %v"
	EL_GOV_DEL_NODE_EXCH_PATTERN_FROM_DB      = "Error deleting node exchange pattern from the local database. %v"

	// exchange
	EL_GOV_ERR_RETRIEVE_NODE_FROM_EXCH = "Error retrieving node %v from the Exchange: %v"
	EL_GOV_ERR_UPDATE_REGSVCS_IN_EXCH  = "Error updating registeredServices for node %v in the Exchange: %v"

	// image
	EL_GOV_IMAGE_LOADED            = "Image loaded for %v/%v."
	EL_GOV_IMAGE_LOADED_FOR_SVC    = "Image loaded for service %v/%v."
	EL_GOV_ERR_LOADING_IMG         = "Error loading image for %v/%v. Reason: %v"
	EL_GOV_ERR_LOADING_IMG_FOR_SVC = "Error loading image for service %v/%v."

	// agreement
	EL_GOV_START_TERM_AG_WITH_REASON    = "Start terminating agreement for %v. Termination reason: %v"
	EL_GOV_AG_REACHED                   = "Agreement reached for service %v. The agreement id is %v."
	EL_GOV_AG_NOT_VALID                 = "Agreement for %v no longer valid on the agbot. Node will cancel it."
	EL_GOV_WL_CONTAINER_UP              = "Workload service containers for %v/%v are up and running."
	EL_GOV_COMPLETE_TERM_AG_WITH_REASON = "Complete terminating agreement for %v. Termination reason: %v"
	EL_GOV_ERR_DEL_AG_IN_EXCH           = "Error deleting agreement for %v in exchange: %v. Will retry."
	EL_GOV_ERR_AG_VERIFICATION          = "Encountered error for AgreementVerification for %v with agbot, error %v"

	// message
	EL_GOV_REPLYACK_WILL_CANCEL_AG            = "ReplyAck indicated that the agbot did not want to pursue the agreement for %v. Node will cancel the agreement"
	EL_GOV_NODE_RECEIVED_CANCEL_MSG           = "Node received Cancel message for %v/%v from agbot %v."
	EL_GOV_ERR_HANDLE_REPLYACK_MSG_FOR_AG     = "Error handling ReplyAck message for %v. %v"
	EL_GOV_ERR_HANDLE_REPLYACK_MSG            = "Error handling ReplyAck message. %v"
	EL_GOV_ERR_HANDLE_DATARECEIVED_MSG_FOR_AG = "Error handling DataReceived message for %v. %v"
	EL_GOV_ERR_HANDLE_DATARECEIVED_MSG        = "Error handling DataReceived message. %v"
	EL_GOV_ERR_HANDLE_METERING_MSG_FOR_AG     = "Error handling MeterNotification message for %v. %v"
	EL_GOV_ERR_HANDLE_METERING_MSG            = "Error handling MeterNotification message. %v"
	EL_GOV_ERR_HANDLE_CANCEL_MSG_FOR_AG       = "Error handling Cancel message for %v. %v"
	EL_GOV_ERR_HANDLE_CANCEL_MSG              = "Error handling Cancel message. %v"

	// service
	EL_GOV_START_WORKLOAD_SVC             = "Start workload service for %v/%v."
	EL_GOV_WORKLOAD_DESTROYED             = "Workload destroyed for %v"
	EL_GOV_SVC_CONTAINER_STARTED          = "Service containers for %v started."
	EL_GOV_COMPLETE_CLEANUP_SVC           = "Complete cleaning up the service instance %v."
	EL_GOV_START_DEPENDENT_SVC            = "Start dependent services for %v/%v."
	EL_GOV_ERR_START_DEPENDENT_SVC        = "Encountered error starting dependen services for %v/%v. %v"
	EL_GOV_ERR_START_DEPENDENT_SVC_FOR_AG = "Error starting dependen service %v/%v version %v for agreement %v. %v"
	EL_GOV_START_CLEANUP_SVC              = "Start cleaning up service %v because agreement %v ended."
	EL_GOV_ERR_START_SVC                  = "Error starting service %v/%v version %v, error: %v"
	EL_GOV_ERR_GET_ALL_SVCS_FROM_AGS      = "Error getting all the services from agreements: %v"

	// agreement-less service
	EL_GOV_START_AGLESS_SVC                           = "Start agreement-less service %v/%v."
	EL_GOV_COMPLETE_START_AGLESS_SVC                  = "Complete starting agreement-less service %v/%v and its dependents."
	EL_GOV_ERR_START_AGLESS_SVC                       = "Unable to start agreement-less service %v/%v, error %v"
	EL_GOV_ERR_START_AGLESS_SVC_ERR_SEARCH_PATTERN    = "Unable to start agreement-less services, error searching for pattern %v in exchange, error: %v"
	EL_GOV_ERR_START_AGLESS_SVC_ERR_PATTERN_NOT_FOUND = "Unable to start agreement-less services, pattern %v not found in exchange"
	EL_GOV_ERR_START_AGLESS_SVC_ERR_SDEF_NOT_FOUND    = "Unable to start agreement-less service %v/%v, local service definition not found"

	// service upgrade
	EL_GOV_START_UPGRADE    = "Start upgrading service %v/%v from version %v to version %v."
	EL_GOV_COMPLETE_UPGRADE = "Complete upgrading service %v/%v from version %v to version %v."
	EL_GOV_FAILED_UPGRADE   = "Failed to upgrade service %v/%v from version %v to version %v, error: %v"

	// service downgrade
	EL_GOV_START_DOWNGRADE_FOR_AG                 = "Start downgrading service %v/%v version %v because service for agreement failed to start."
	EL_GOV_START_DOWNGRADE                        = "Start downgrading service %v/%v version %v because service failed to start."
	EL_GOV_START_DOWNGRADE_BECAUSE_UPGRADE_FAILED = "Start downgrading service %v/%v version %v because upgrading failed."
	EL_GOV_FAILED_DOWNGRADE                       = "Failed to downgrade service %v/%v version %v, error: %v"
	EL_GOV_COMPLETE_DOWNGRADE                     = "Completed downgrading service %v/%v from version %v to version %v."
	EL_GOV_ERR_FIND_SDEF_FOR_DOWNGRADE            = "Error finding the new service definition to downgrade to for %v/%v version %v key %v. error: %v"
	EL_GOV_ERR_NO_VERSION_TO_DOWNGRADE            = "Could not find lower version to downgrade for %v/%v version %v."
	EL_GOV_ERR_DOWNGRADE_FROM                     = "Error downgrading service %v/%v from version %v to version %v. Error: %v"
	EL_GOV_ERR_DOWNGRADE                          = "Error downgrading service %v/%v version %v. %v"

	// service retry
	EL_GOV_START_SVC_RETRY            = "Start retrying number %v for dependent service %v version %v because service failed."
	EL_GOV_FAILED_SVC_RETRY           = "Failed retrying number %v for dependent service %v version %v."
	EL_GOV_ERR_GET_SVC_RETRY_CNT      = "Failed to get the service retry count for %v version %v. %v"
	EL_GOV_ERR_UPDATE_SVC_RETRY_STATE = "Error updating retry start state for service instance %v in dadabase. %v"

	// pattern change
	EL_GOV_EXCH_NODE_PATTERN_CHANGED       = "Node pattern changed on the Exchange from %v to %v."
	EL_GOV_ERR_REG_NODE_WITH_NEW_PATTERN   = "Encountered error while re-registering node with new pattern %v. %v"
	EL_GOV_START_REREG_NODE_PATTERN_CHANGE = "Start re-registering node after pattern changed to %v."
	EL_GOV_END_REREG_NODE_PATTERN_CHANGE   = "Complete re-registering node after pattern changed to %v."
	EL_GOV_PATTERN_CHANGED_AGAIN           = "Node pattern changed again on the Exchange. Will register the node with the new pattern: %v"
	EL_GOV_ERR_VALIDATE_NEW_PATTERN        = "Error validating new node pattern %v: %v"
	EL_GOV_NODE_KEEP_OLD_PATTERN           = "The node will keep using the old pattern %v"
	EL_GOV_NEW_PATTERN_VERIFIED            = "New pattern %v is verified. Will cancel agreements and re-register the node with the new pattern."
)

// This is does nothing useful at run time.
// This code is only used in compileing time to make the eventlog messages gets into the catalog so that
// they can be translated.
// The event log messages will be saved in English. But the CLI can request them in different languages.
func MarkI18nMessages() {
	// get message printer. anax default language is English
	msgPrinter := i18n.GetMessagePrinter()

	//db
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_AG_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_MNM)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_CANM)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_RAM)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_AG_FROM_DB_FOR_DRM)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_UNARCHIVED_AG_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_MARK_AG_TERMINATED_IN_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_SDEFS_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_SINSTS_VER_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_SINSTS_FOR_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_SINSTS_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_SINST_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_MATCH_AGS_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_SAVE_NODE_CONFIGSTATE_TO_DB)
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_DEVICE_FROM_DB)
	msgPrinter.Sprintf(EL_GOV_DEL_NODE_EXCH_PATTERN_FROM_DB)

	// exchange
	msgPrinter.Sprintf(EL_GOV_ERR_RETRIEVE_NODE_FROM_EXCH)
	msgPrinter.Sprintf(EL_GOV_ERR_UPDATE_REGSVCS_IN_EXCH)

	// image
	msgPrinter.Sprintf(EL_GOV_IMAGE_LOADED)
	msgPrinter.Sprintf(EL_GOV_IMAGE_LOADED_FOR_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_LOADING_IMG)
	msgPrinter.Sprintf(EL_GOV_ERR_LOADING_IMG_FOR_SVC)

	// agreement
	msgPrinter.Sprintf(EL_GOV_START_TERM_AG_WITH_REASON)
	msgPrinter.Sprintf(EL_GOV_AG_REACHED)
	msgPrinter.Sprintf(EL_GOV_AG_NOT_VALID)
	msgPrinter.Sprintf(EL_GOV_WL_CONTAINER_UP)
	msgPrinter.Sprintf(EL_GOV_COMPLETE_TERM_AG_WITH_REASON)
	msgPrinter.Sprintf(EL_GOV_ERR_DEL_AG_IN_EXCH)
	msgPrinter.Sprintf(EL_GOV_ERR_AG_VERIFICATION)

	// message
	msgPrinter.Sprintf(EL_GOV_REPLYACK_WILL_CANCEL_AG)
	msgPrinter.Sprintf(EL_GOV_NODE_RECEIVED_CANCEL_MSG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_REPLYACK_MSG_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_REPLYACK_MSG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_DATARECEIVED_MSG_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_DATARECEIVED_MSG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_METERING_MSG_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_METERING_MSG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_CANCEL_MSG_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_ERR_HANDLE_CANCEL_MSG)

	// service
	msgPrinter.Sprintf(EL_GOV_START_WORKLOAD_SVC)
	msgPrinter.Sprintf(EL_GOV_WORKLOAD_DESTROYED)
	msgPrinter.Sprintf(EL_GOV_SVC_CONTAINER_STARTED)
	msgPrinter.Sprintf(EL_GOV_COMPLETE_CLEANUP_SVC)
	msgPrinter.Sprintf(EL_GOV_START_DEPENDENT_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_START_DEPENDENT_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_START_DEPENDENT_SVC_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_START_CLEANUP_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_START_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_GET_ALL_SVCS_FROM_AGS)

	// agreement-less service
	msgPrinter.Sprintf(EL_GOV_START_AGLESS_SVC)
	msgPrinter.Sprintf(EL_GOV_COMPLETE_START_AGLESS_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_START_AGLESS_SVC)
	msgPrinter.Sprintf(EL_GOV_ERR_START_AGLESS_SVC_ERR_SEARCH_PATTERN)
	msgPrinter.Sprintf(EL_GOV_ERR_START_AGLESS_SVC_ERR_PATTERN_NOT_FOUND)
	msgPrinter.Sprintf(EL_GOV_ERR_START_AGLESS_SVC_ERR_SDEF_NOT_FOUND)

	// service upgrade
	msgPrinter.Sprintf(EL_GOV_START_UPGRADE)
	msgPrinter.Sprintf(EL_GOV_COMPLETE_UPGRADE)
	msgPrinter.Sprintf(EL_GOV_FAILED_UPGRADE)

	// service downgrade
	msgPrinter.Sprintf(EL_GOV_START_DOWNGRADE_FOR_AG)
	msgPrinter.Sprintf(EL_GOV_START_DOWNGRADE)
	msgPrinter.Sprintf(EL_GOV_START_DOWNGRADE_BECAUSE_UPGRADE_FAILED)
	msgPrinter.Sprintf(EL_GOV_FAILED_DOWNGRADE)
	msgPrinter.Sprintf(EL_GOV_COMPLETE_DOWNGRADE)
	msgPrinter.Sprintf(EL_GOV_ERR_FIND_SDEF_FOR_DOWNGRADE)
	msgPrinter.Sprintf(EL_GOV_ERR_NO_VERSION_TO_DOWNGRADE)
	msgPrinter.Sprintf(EL_GOV_ERR_DOWNGRADE_FROM)
	msgPrinter.Sprintf(EL_GOV_ERR_DOWNGRADE)

	// service retry
	msgPrinter.Sprintf(EL_GOV_START_SVC_RETRY)
	msgPrinter.Sprintf(EL_GOV_FAILED_SVC_RETRY)
	msgPrinter.Sprintf(EL_GOV_ERR_GET_SVC_RETRY_CNT)
	msgPrinter.Sprintf(EL_GOV_ERR_UPDATE_SVC_RETRY_STATE)

	// pattern change
	msgPrinter.Sprintf(EL_GOV_EXCH_NODE_PATTERN_CHANGED)
	msgPrinter.Sprintf(EL_GOV_ERR_REG_NODE_WITH_NEW_PATTERN)
	msgPrinter.Sprintf(EL_GOV_START_REREG_NODE_PATTERN_CHANGE)
	msgPrinter.Sprintf(EL_GOV_END_REREG_NODE_PATTERN_CHANGE)
	msgPrinter.Sprintf(EL_GOV_PATTERN_CHANGED_AGAIN)
	msgPrinter.Sprintf(EL_GOV_ERR_VALIDATE_NEW_PATTERN)
	msgPrinter.Sprintf(EL_GOV_NODE_KEEP_OLD_PATTERN)
	msgPrinter.Sprintf(EL_GOV_NEW_PATTERN_VERIFIED)
}
