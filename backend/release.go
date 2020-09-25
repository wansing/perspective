package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func release(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
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

	defer r.SeeOther("/edit/%d%s", versionNo, selected.HrefPath())

	state, err := selected.ReleaseState(selectedVersion, r.User)
	if err != nil {
		return err
	}

	if !state.CanEditNode() {
		return ErrAuth
	}

	releaseToGroup := state.ReleaseToGroup()
	if releaseToGroup == nil {
		return errors.New("no release group")
	}

	if err = r.db.SetWorkflowGroup(selected, selectedVersion, (*releaseToGroup).Id()); err != nil {
		return err
	}

	return nil
}
