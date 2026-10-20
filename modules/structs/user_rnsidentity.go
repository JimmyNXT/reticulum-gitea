// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// RNSIdentity a Reticulum identity hash associated with a user
type RNSIdentity struct {
	// The unique identifier of the Reticulum identity
	ID int64 `json:"id"`
	// The display name of the Reticulum identity
	Name string `json:"name"`
	// The 32 character hexadecimal Reticulum identity hash
	IdentityHash string `json:"identity_hash"`
	// swagger:strfmt date-time
	// The date and time when the Reticulum identity was created
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	// The date and time when the Reticulum identity was last updated
	Updated time.Time `json:"updated_at"`
	// The owner of the Reticulum identity
	Owner *User `json:"owner"`
}

// CreateRNSIdentityOption options for adding a user Reticulum identity
type CreateRNSIdentityOption struct {
	// A name for the Reticulum identity
	//
	// required: true
	Name string `json:"name" binding:"Required;MaxSize(50)"`
	// The 32 character hexadecimal Reticulum identity hash
	//
	// required: true
	IdentityHash string `json:"identity_hash" binding:"Required"`
}
