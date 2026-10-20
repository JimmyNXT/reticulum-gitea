// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func listRNSIdentities(ctx *context.APIContext, uid int64, listOptions db.ListOptions) {
	rnsIdentities, count, err := db.FindAndCount[asymkey_model.RNSIdentity](ctx, asymkey_model.FindRNSIdentityOptions{
		ListOptions: listOptions,
		OwnerID:     uid,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiRNSIdentities := make([]*api.RNSIdentity, len(rnsIdentities))
	for i := range rnsIdentities {
		apiRNSIdentities[i] = convert.ToRNSIdentity(ctx, rnsIdentities[i], ctx.Doer)
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, &apiRNSIdentities)
}

// ListRNSIdentities list the Reticulum identities of a user
func ListRNSIdentities(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/rns_identities user userListRNSIdentities
	// ---
	// summary: List the Reticulum identities of a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSIdentityList"
	listRNSIdentities(ctx, ctx.ContextUser.ID, utils.GetListOptions(ctx))
}

// ListMyRNSIdentities list the Reticulum identities of the authenticated user
func ListMyRNSIdentities(ctx *context.APIContext) {
	// swagger:operation GET /user/rns_identities user userCurrentListRNSIdentities
	// ---
	// summary: List the Reticulum identities of the authenticated user
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSIdentityList"
	listRNSIdentities(ctx, ctx.Doer.ID, utils.GetListOptions(ctx))
}

// GetRNSIdentity get a Reticulum identity of the authenticated user
func GetRNSIdentity(ctx *context.APIContext) {
	// swagger:operation GET /user/rns_identities/{id} user userCurrentGetRNSIdentity
	// ---
	// summary: Get a Reticulum identity of the authenticated user
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the Reticulum identity
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSIdentity"
	//   "404":
	//     "$ref": "#/responses/notFound"
	rnsIdentity, err := asymkey_model.GetRNSIdentityForUserByID(ctx, ctx.Doer.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRNSIdentity(ctx, rnsIdentity, ctx.Doer))
}

// CreateUserRNSIdentity creates a Reticulum identity for the given user
func CreateUserRNSIdentity(ctx *context.APIContext, form api.CreateRNSIdentityOption, uid int64) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageRNSIdentities) {
		ctx.APIErrorNotFound("rns identities setting is not allowed to be changed")
		return
	}

	rnsIdentity, err := asymkey_model.AddRNSIdentity(ctx, uid, form.Name, form.IdentityHash)
	if err != nil {
		HandleAddRNSIdentityError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToRNSIdentity(ctx, rnsIdentity, ctx.Doer))
}

// swagger:parameters userCurrentPostRNSIdentity
type swaggerUserCurrentPostRNSIdentity struct {
	// in: body
	Form api.CreateRNSIdentityOption
}

// CreateRNSIdentity creates a Reticulum identity for the authenticated user
func CreateRNSIdentity(ctx *context.APIContext) {
	// swagger:operation POST /user/rns_identities user userCurrentPostRNSIdentity
	// ---
	// summary: Create a Reticulum identity for the authenticated user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateRNSIdentityOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/RNSIdentity"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm[*api.CreateRNSIdentityOption](ctx)
	CreateUserRNSIdentity(ctx, *form, ctx.Doer.ID)
}

// DeleteRNSIdentity deletes a Reticulum identity of the authenticated user
func DeleteRNSIdentity(ctx *context.APIContext) {
	// swagger:operation DELETE /user/rns_identities/{id} user userCurrentDeleteRNSIdentity
	// ---
	// summary: Delete a Reticulum identity of the authenticated user
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the Reticulum identity
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageRNSIdentities) {
		ctx.APIErrorNotFound("rns identities setting is not allowed to be changed")
		return
	}

	if err := asymkey_model.DeleteRNSIdentity(ctx, ctx.Doer, ctx.PathParamInt64("id")); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// HandleAddRNSIdentityError handle error from AddRNSIdentity
func HandleAddRNSIdentityError(ctx *context.APIContext, err error) {
	switch {
	case asymkey_model.IsErrRNSIdentityAlreadyUsed(err):
		ctx.APIError(http.StatusUnprocessableEntity, "A Reticulum identity with the same hash already exists")
	case asymkey_model.IsErrKeyNameAlreadyUsed(err):
		ctx.APIError(http.StatusUnprocessableEntity, "A Reticulum identity with the same name already exists")
	case asymkey_model.IsErrRNSInvalidIdentityHash(err):
		ctx.APIError(http.StatusUnprocessableEntity, "The identity hash must be 32 hexadecimal characters")
	default:
		ctx.APIErrorInternal(err)
	}
}
