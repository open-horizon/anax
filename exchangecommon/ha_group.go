package exchangecommon

import ()

type HAGroup struct {
	Description string   `json:"description"`
	Name        string   `json:"name"`    // the name of the HA group
	Members     []string `json:"members"` // all the nodes in this HA group.
	LastUpdated string   `json:"lastUpdated,omitempty"`
}

type GetHAGroupResponse struct {
	NodeGroups []HAGroup `json:"nodeGroups"`
}

type HAGroupPutPostRequest struct {
	Description string   `json:"description,omitempty"`
	Members     []string `json:"members,omitempty"` // all the nodes in this HA group.
}
