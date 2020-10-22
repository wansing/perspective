package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func revoke(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	versionNo, _ := strconv.Atoi(params.ByName("version"))
	if versionNo == 0 {
		versionNo = selected.MaxVersionNo()
	}

	selectedVersion, err := selected.GetVersion(versionNo)
	if err != nil {
		return err
	}

	defer ctx.SeeOther("/edit/%d%s", versionNo, selected.Location())

	state, err := selected.ReleaseState(selectedVersion, ctx.User)
	if err != nil {
		return err
	}

	if !state.CanEditNode() {
		return ErrAuth
	}

	revokeToGroup := state.RevokeToGroup()
	if revokeToGroup == nil {
		return errors.New("no revoke group")
	}

	if err = ctx.db.SetWorkflowGroup(selected, selectedVersion, (*revokeToGroup).ID()); err != nil {
		return err
	}

	return nil
}
