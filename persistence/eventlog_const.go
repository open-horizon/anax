package persistence

import ()

// serverity for eventlog
const (
	SEVERITY_INFO  = "info"
	SEVERITY_WARN  = "warning"
	SEVERITY_ERROR = "error"
	SEVERITY_FATAL = "fatal"
)

// source type for eventlog
const (
	SRC_TYPE_AG   = "agreement"
	SRC_TYPE_SVC  = "service"
	SRC_TYPE_NODE = "node"
	SRC_TYPE_DB   = "database"
	SRC_TYPE_EXCH = "exchange"
)

// event code for eventlog
const (
	// general errors
	EC_DATABASE_ERROR       = "database_error"
	EC_API_USER_INPUT_ERROR = "api_user_input_error"
	EC_EXCHANGE_ERROR       = "exchange_error"

	// initialization
	EC_ERROR_CONTAINER_SYNC_ON_INIT = "error_container_sync_on_init"
	EC_ERROR_AGREEMENT_SYNC_ON_INIT = "error_agreement_sync_on_init"
	EC_ERROR_ACCESS_STORAGE_DIR     = "error_access_storage_dir"
	EC_ERROR_CREATE_IPTABLE_CLIENT  = "error_create_iptable_client"
	EC_ERROR_CREATE_DOCKER_CLIENT   = "error_create_docker_client"

	// node configuration/registration
	EC_START_NODE_CONFIG_REG    = "start_node_configuration_registration"
	EC_NODE_CONFIG_REG_COMPLETE = "node_configuration_registration_complete"
	EC_ERROR_NODE_CONFIG_REG    = "error_node_configuration_registration"

	// node update
	EC_START_NODE_UPDATE    = "start_node_update"
	EC_NODE_UPDATE_COMPLETE = "node_update_complete"
	EC_ERROR_NODE_UPDATE    = "error_node_update"

	// node pattern
	EC_NODE_PATTERN_CHANGED            = "node_pattern_changed"
	EC_NODE_PATTERN_CHANGED_AGAIN      = "node_pattern_changed_again"
	EC_START_REREG_NODE_PATTERN_CHANGE = "start_rereg_node_pattern_change"
	EC_END_REREG_NODE_PATTERN_CHANGE   = "end_rereg_node_pattern_change"
	EC_ERROR_REG_NODE_WITH_NEW_PATTERN = "error_reg_node_with_new_pattern"
	EC_ERROR_VALIDATE_NEW_PATTERN      = "error_validate_new_pattern"
	EC_NODE_KEEP_OLD_PATTERN           = "node_keep_old_pattern"
	EC_NEW_PATTERN_VERIFIED            = "new_pattern_verified"

	// node unreggistratin
	EC_START_NODE_UNREG    = "start_node_unregistration"
	EC_NODE_UNREG_COMPLETE = "node_unregistration_complete"
	EC_ERROR_NODE_UNREG    = "error_node_unregistration"

	// node heartbeat
	EC_NODE_HEARTBEAT_FAILED   = "node_heartbeat_failed"
	EC_NODE_HEARTBEAT_RESTORED = "node_heartbeat_restored"

	// service configuration
	EC_START_SERVICE_CONFIG                = "start_service_configuration"
	EC_SERVICE_CONFIG_COMPLETE             = "service_configuration_complete"
	EC_ERROR_SERVICE_CONFIG                = "error_service_configuration"
	EC_WARNING_SERVICE_CONFIG              = "warning_service_configuration"
	EC_SERVICE_CONFIG_IGNORE_TYPE_MISMATCH = "ignore_type_mismatch"

	// service config state
	EC_START_CHANGING_SERVICE_CONFIGSTATE    = "start_changing_service_configuration_state"
	EC_CHANGING_SERVICE_CONFIGSTATE_COMPLETE = "changing_service_configuration_state_complete"
	EC_ERROR_CHANGING_SERVICE_CONFIGSTATE    = "error_changing_service_configuration_state"

	// agreement related event code
	EC_RECEIVED_PROPOSAL         = "received_proposal"
	EC_IGNORE_PROPOSAL           = "ignore_proposal"
	EC_REJECT_PROPOSAL           = "reject_proposal"
	EC_ERROR_IN_PROPOSAL         = "error_in_proposal"
	EC_ERROR_PROCESSING_PROPOSAL = "error_processing_proposal"

	EC_RECEIVED_REPLYACK_MESSAGE         = "received_replyack_message"
	EC_IGNORE_REPLYACK_MESSAGE           = "ignore_replyack_message"
	EC_ERROR_PROCESSING_REPLYACT_MESSAGE = "error_ptocessing_replyack_message"

	EC_ERROR_PROCESSING_DATARECEIVED_MESSAGE = "error_processing_datareceived_message"

	EC_ERROR_PROCESSING_METERING_NOTIFY_MESSAGE = "error_processing_metering_notify_message"

	EC_RECEIVED_CANCEL_AGREEMENT_MESSAGE         = "received_cancel_agreement_message"
	EC_ERROR_PROCESSING_CANCEL_AGREEMENT_MESSAGE = "error_processing_cancel_agreement_message"

	EC_ERROR_NODE_IS_OFFLINE = "error_node_is_offline"
	EC_ERROR_NODE_SYNC       = "error_node_sync"

	EC_NODE_POLICY_UPDATED         = "update_node_policy"
	EC_NODE_POLICY_DELETED         = "delete_node_policy"
	EC_ERROR_NODE_POLICY_UPDATE    = "error_policy_update"
	EC_ERROR_NODE_POLICY_PATCH     = "error_policy_patch"
	EC_START_POLICY_ADVERTISING    = "start_policy_advertising"
	EC_COMPLETE_POLICY_ADVERTISING = "complete_policy_advertising"
	EC_ERROR_POLICY_ADVERTISING    = "error_policy_advertising"

	EC_NODE_USERINPUT_UPDATED      = "update_node_userinput"
	EC_NODE_USERINPUT_DELETED      = "delete_node_userinput"
	EC_ERROR_NODE_USERINPUT_UPDATE = "error_userinput_update"
	EC_ERROR_NODE_USERINPUT_PATCH  = "error_userinput_patch"

	EC_AGREEMENT_REACHED                  = "agreement_reached"
	EC_CANCEL_AGREEMENT                   = "cancel_agreement"
	EC_AGREEMENT_CANCELED                 = "agreement_canceled"
	EC_CANCEL_AGREEMENT_EXECUTION_TIMEOUT = "cancel_agreement_execution_timeout"
	EC_CANCEL_AGREEMENT_NO_REPLYACK       = "cancel_agreement_no_replyack"
	EC_CANCEL_AGREEMENT_PER_AGBOT         = "cancel_agreement_per_agbot_request"
	EC_CANCEL_AGREEMENT_SERVICE_SUSPENDED = "cancel_agreement_service_suspended"
	EC_CANCEL_AGREEMENT_POLICY_CHANGED    = "cancel_agreement_policy_changed"

	EC_CONTAINER_RUNNING          = "container_running"
	EC_CONTAINER_STOPPED          = "container_stopped"
	EC_ERROR_IN_DEPLOYMENT_CONFIG = "error_in_deployment_configuration"
	EC_ERROR_START_CONTAINER      = "error_start_container"

	EC_IMAGE_LOADED                       = "image_loaded"
	EC_ERROR_IMAGE_LOADE                  = "error_image_load"
	EC_ERROR_AGREEMENT_VERIFICATION       = "error_in_agreement_verification"
	EC_ERROR_DELETE_AGREEMENT_IN_EXCHANGE = "error_delete_agreement_in_exchange"

	// event code for services
	EC_START_SERVICE            = "start_service"
	EC_ERROR_START_SERVICE      = "error_start_service"
	EC_COMPLETE_SERVICE_STARTUP = "complete_service_startup"

	EC_START_DEPENDENT_SERVICE             = "start_dependent_service"
	EC_ERROR_START_DEPENDENT_SERVICE       = "error_start_dependent_service"
	EC_DEPENDENT_SERVICE_FAILED            = "dependent_service_failed"
	EC_COMPLETE_DEPENDENT_SERVICE          = "complete_dependent_service"
	EC_REMOVE_OLD_DEPENDENT_SERVICE_FAILED = "remove_old_dependent_service_failed"

	EC_START_RETRY_DEPENDENT_SERVICE       = "start_retry_dependent_service"
	EC_ERROR_START_RETRY_DEPENDENT_SERVICE = "error_start_retry_dependent_service"
	EC_DEPENDENT_SERVICE_RETRY_FAILED      = "dependent_service_retry_failed"
	EC_COMPLETE_RETRY_DEPENDENT_SERVICE    = "complete_retry_dependent_service"

	EC_START_AGREEMENTLESS_SERVICE            = "start_agreementless_service"
	EC_ERROR_START_AGREEMENTLESS_SERVICE      = "error_start_agreementless_service"
	EC_COMPLETE_AGREEMENTLESS_SERVICE_STARTUP = "complete_agreementless_service_startup"

	EC_START_DOWNGRADE_SERVICE    = "start_downgrade_service"
	EC_COMPLETE_DOWNGRADE_SERVICE = "complete_downgrade_service"
	EC_ERROR_DOWNGRADE_SERVICE    = "error_downgrade_service"
	EC_NO_VERSION_TO_DOWNGRADE    = "no_version_to_downgrade"

	EC_START_UPGRADE_SERVICE    = "start_rollback_service"
	EC_COMPLETE_UPGRADE_SERVICE = "complete_rollback_service"
	EC_ERROR_UPGRADE_SERVICE    = "error_rollback_service"

	EC_START_CLEANUP_SERVICE    = "start_cleanup_service"
	EC_COMPLETE_CLEANUP_SERVICE = "complete_cleanup_service"
	EC_ERROR_CLEANUP_SERVICE    = "error_cleanup_service"
)
