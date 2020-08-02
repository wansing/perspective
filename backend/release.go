package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func release(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if req.Method != http.MethodPost {
		return errors.New("POST requests only")
	}

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	defer r.SeeOther(hrefBackendVersion("edit", selected, selected.VersionNo()))

	state, err := selected.ReleaseState(r.User)
	if err != nil {
		return err
	}

	if !state.CanEdit() {
		return ErrAuth
	}

	releaseToGroup := state.ReleaseToGroup()
	if releaseToGroup == nil {
		return errors.New("no release group")
	}

	if err = r.db.SetWorkflowGroup(selected, (*releaseToGroup).Id()); err != nil {
		return err
	}

	return nil
}
