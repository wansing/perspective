package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func revoke(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if req.Method != http.MethodPost {
		return errors.New("POST requests only")
	}

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	defer r.SeeOther("/edit%s:%d", selected.HrefPath(), selected.VersionNo())

	state, err := selected.ReleaseState(r.User)
	if err != nil {
		return err
	}

	if !state.CanEdit() {
		return ErrAuth
	}

	revokeToGroup := state.RevokeToGroup()
	if revokeToGroup == nil {
		return errors.New("no revoke group")
	}

	if err = r.db.SetWorkflowGroup(selected, (*revokeToGroup).Id()); err != nil {
		return err
	}

	return nil
}
