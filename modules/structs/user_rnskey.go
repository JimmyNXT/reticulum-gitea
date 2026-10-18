// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// RNSKey represents a Reticulum identity key
type RNSKey struct {
	// The unique identifier of the RNS key
	ID int64 `json:"id"`
	// The name of the RNS key
	Name string `json:"name"`
	// The Reticulum identity hash
	IdentityHash string `json:"identity_hash"`
	// swagger:strfmt date-time
	// The date and time when the RNS key was created
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	// The date and time when the RNS key was updated
	Updated time.Time `json:"updated_at"`
	// The user who owns this RNS key
	Owner *User `json:"owner"`
}

// CreateRNSKeyOption options to create a Reticulum identity key
type CreateRNSKeyOption struct {
	// The name of the RNS key
	//
	// required: true
	// max size: 50
	Name string `json:"name" binding:"Required;MaxSize(50)"`
	// The Reticulum identity hash (32 hexadecimal characters)
	//
	// required: true
	IdentityHash string `json:"identity_hash" binding:"Required"`
}
