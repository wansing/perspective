package core

// A ReleaseState contains the state of a specific node and version in their workflow, when edited by a specific user.
//
// Currently it follows the subset model (see the package comment). Its functions might become an interface for different workflow models.
type ReleaseState struct {
	workflow *Workflow
	index    int       // index of current workflow group in groups
	groups   []DBGroup // cached, last element is Readers{}
	isMember []bool    // cached, refers to groups
}

func GetReleaseState(workflow *Workflow, workflowGroupID int, user DBUser) (*ReleaseState, error) {

	wfGroups, err := workflow.Groups()
	if err != nil {
		return nil, err
	}

	var groups = append(wfGroups, Readers{})

	var index = 0 // in case of an inconsistency, the first group is default
	for i, group := range groups {
		if group.ID() == workflowGroupID {
			index = i
			break
		}
	}

	var isMember = make([]bool, len(groups)) // Readers won't matter here because they are an empty group
	for i := range groups {
		isMember[i], err = groups[i].HasMember(user)
		if err != nil {
			return nil, err
		}
	}

	return &ReleaseState{
		workflow: workflow,
		groups:   groups,
		index:    index,
		isMember: isMember,
	}, nil
}

func (rs *ReleaseState) CanEditNode() bool {
	for _, is := range rs.isMember {
		if is {
			return true
		}
	}
	return false
}

// RevokeToGroup returns the group to which the user can revoke the node directly, or nil.
// In the subset model, this is latest "save group" before the current workflow group.
func (rs *ReleaseState) RevokeToGroup() *DBGroup {
	var revokeGroup *DBGroup
	for i := 0; i < rs.index; i++ { // groups before the current workflow group
		if rs.isMember[i] { // user must be a member of the group
			revokeGroup = &rs.groups[i] // store it, so we get the latest possible group
		}
	}
	return revokeGroup
}

// RevokeToGroup returns the group to which the user can release the node directly, or nil.
// In the subset model, this is first "save group" after the current workflow group.
func (rs *ReleaseState) ReleaseToGroup() *DBGroup {
	for i := rs.index + 1; i < len(rs.groups); i++ { // groups after the current workflow group
		if i > 0 && rs.isMember[i-1] { // user must be a member of the previous group
			return &rs.groups[i] // return it, so we get the earliest possible group
		}
	}
	return nil
}

// SaveGroups determines the "save groups" of current user, which are the values that she can assign to workflowGroup.
// This is every group which she is a member of, and each subsequent group.
func (rs *ReleaseState) SaveGroups() []DBGroup {
	var saveGroups = []DBGroup{}
	for i, g := range rs.groups {
		if g.ID() != 0 && rs.isMember[i] {
			saveGroups = append(saveGroups, g)
			continue
		}
		if i > 0 && rs.isMember[i-1] {
			saveGroups = append(saveGroups, g)
			continue
		}
	}
	return saveGroups
}

// IsSaveGroup returns whether a given group id is a "save group".
func (rs *ReleaseState) IsSaveGroup(groupID int) bool {
	for _, sg := range rs.SaveGroups() {
		if sg.ID() == groupID {
			return true
		}
	}
	return false
}

// SuggestedSaveGroup determines the suggested "save group" for new versions.
func (rs *ReleaseState) SuggestedSaveGroup() *DBGroup {

	// one of saveGroups
	var saveGroups = rs.SaveGroups()

	if len(saveGroups) == 0 {
		return nil
	}

	// except Readers{}, which is the last slice element
	saveGroups = saveGroups[:len(saveGroups)-1]

	// prefer current workflow group
	for _, g := range saveGroups {
		if g.ID() == rs.WorkflowGroup().ID() {
			return &g
		}
	}

	// else suggest the first save group
	return &saveGroups[0]
}

func (rs *ReleaseState) Workflow() *Workflow {
	return rs.workflow
}

func (rs *ReleaseState) WorkflowGroup() DBGroup {
	return rs.groups[rs.index]
}
