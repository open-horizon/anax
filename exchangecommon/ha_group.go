package exchangecommon

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

func (e HAGroup) DeepCopy() *HAGroup {
	hagroupCopy := HAGroup{Description: e.Description, Name: e.Name, LastUpdated: e.LastUpdated}

	if e.Members == nil {
		hagroupCopy.Members = nil
	} else {
		for _, member := range e.Members {
			hagroupCopy.Members = append(hagroupCopy.Members, member)
		}
	}
	return &hagroupCopy
}
