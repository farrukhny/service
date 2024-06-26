// Package userapi maintains the web based api for user access.
package userapi

import (
	"context"
	"errors"
	"net/http"
	"net/mail"

	"github.com/ardanlabs/service/app/api/errs"
	"github.com/ardanlabs/service/app/core/crud/userapp"
	"github.com/ardanlabs/service/foundation/validate"
	"github.com/ardanlabs/service/foundation/web"
)

type api struct {
	userApp *userapp.Core
}

func newAPI(userApp *userapp.Core) *api {
	return &api{
		userApp: userApp,
	}
}

func (api *api) create(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var app userapp.NewUser
	if err := web.Decode(r, &app); err != nil {
		return errs.New(errs.FailedPrecondition, err)
	}

	usr, err := api.userApp.Create(ctx, app)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusCreated)
}

func (api *api) update(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var app userapp.UpdateUser
	if err := web.Decode(r, &app); err != nil {
		return errs.New(errs.FailedPrecondition, err)
	}

	usr, err := api.userApp.Update(ctx, app)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusOK)
}

func (api *api) updateRole(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var app userapp.UpdateUserRole
	if err := web.Decode(r, &app); err != nil {
		return errs.New(errs.FailedPrecondition, err)
	}

	usr, err := api.userApp.UpdateRole(ctx, app)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusOK)
}

func (api *api) delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if err := api.userApp.Delete(ctx); err != nil {
		return err
	}

	return web.Respond(ctx, w, nil, http.StatusNoContent)
}

func (api *api) query(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	qp, err := parseQueryParams(r)
	if err != nil {
		return err
	}

	usr, err := api.userApp.Query(ctx, qp)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusOK)
}

func (api *api) queryByID(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	usr, err := api.userApp.QueryByID(ctx)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusOK)
}

func (api *api) token(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	kid := web.Param(r, "kid")
	if kid == "" {
		return validate.NewFieldsError("kid", errors.New("missing kid"))
	}

	email, pass, ok := r.BasicAuth()
	if !ok {
		return errs.Newf(errs.Unauthenticated, "authorize: must provide email and password in Basic auth")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return errs.Newf(errs.Unauthenticated, "authorize: invalid email format")
	}

	usr, err := api.userApp.Token(ctx, kid, *addr, pass)
	if err != nil {
		return err
	}

	return web.Respond(ctx, w, usr, http.StatusOK)
}
