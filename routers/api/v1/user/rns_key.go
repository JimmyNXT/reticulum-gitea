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

func listRNSKeys(ctx *context.APIContext, uid int64, listOptions db.ListOptions) {
	keys, total, err := db.FindAndCount[asymkey_model.RNSKey](ctx, asymkey_model.FindRNSKeyOptions{
		ListOptions: listOptions,
		OwnerID:     uid,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiKeys := make([]*api.RNSKey, len(keys))
	for i := range keys {
		apiKeys[i] = convert.ToRNSKey(ctx, keys[i], ctx.Doer)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &apiKeys)
}

// ListRNSKeys get the RNS key list of a user
func ListRNSKeys(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/rns_keys user userListRNSKeys
	// ---
	// summary: List the given user's Reticulum identities
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user whose Reticulum identity list is to be obtained
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSKeyList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	listRNSKeys(ctx, ctx.ContextUser.ID, utils.GetListOptions(ctx))
}

// ListMyRNSKeys get the RNS key list of the authenticated user
func ListMyRNSKeys(ctx *context.APIContext) {
	// swagger:operation GET /user/rns_keys user userCurrentListRNSKeys
	// ---
	// summary: List the authenticated user's Reticulum identities
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSKeyList"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listRNSKeys(ctx, ctx.Doer.ID, utils.GetListOptions(ctx))
}

// GetRNSKey get the RNS key based on an id
func GetRNSKey(ctx *context.APIContext) {
	// swagger:operation GET /user/rns_keys/{id} user userCurrentGetRNSKey
	// ---
	// summary: Get a Reticulum identity
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RNSKey"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	key, err := asymkey_model.GetRNSKeyForUserByID(ctx, ctx.Doer.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRNSKey(ctx, key, ctx.Doer))
}

// CreateUserRNSKey creates a new RNS key for the given user by ID.
func CreateUserRNSKey(ctx *context.APIContext, form api.CreateRNSKeyOption, uid int64) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageRNSKeys) {
		ctx.APIErrorNotFound("rns keys setting is not allowed to be changed")
		return
	}

	key, err := asymkey_model.AddRNSKey(ctx, uid, form.Name, form.IdentityHash)
	if err != nil {
		HandleAddRNSKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToRNSKey(ctx, key, ctx.Doer))
}

// swagger:parameters userCurrentPostRNSKey
type swaggerUserCurrentPostRNSKey struct {
	// in:body
	Form api.CreateRNSKeyOption
}

// CreateRNSKey create an RNS key belonging to the authenticated user
func CreateRNSKey(ctx *context.APIContext) {
	// swagger:operation POST /user/rns_keys user userCurrentPostRNSKey
	// ---
	// summary: Create a Reticulum identity
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// responses:
	//   "201":
	//     "$ref": "#/responses/RNSKey"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm[*api.CreateRNSKeyOption](ctx)
	CreateUserRNSKey(ctx, *form, ctx.Doer.ID)
}

// DeleteRNSKey remove an RNS key belonging to the authenticated user
func DeleteRNSKey(ctx *context.APIContext) {
	// swagger:operation DELETE /user/rns_keys/{id} user userCurrentDeleteRNSKey
	// ---
	// summary: Remove a Reticulum identity
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of key to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageRNSKeys) {
		ctx.APIErrorNotFound("rns keys setting is not allowed to be changed")
		return
	}

	if err := asymkey_model.DeleteRNSKey(ctx, ctx.Doer, ctx.PathParamInt64("id")); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// HandleAddRNSKeyError handle add RNSKey error
func HandleAddRNSKeyError(ctx *context.APIContext, err error) {
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
