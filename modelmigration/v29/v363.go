// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

type RNSIdentity struct {
	ID           int64              `xorm:"pk autoincr"`
	OwnerID      int64              `xorm:"INDEX NOT NULL"`
	Name         string             `xorm:"VARCHAR(50) NOT NULL"`
	IdentityHash string             `xorm:"INDEX UNIQUE NOT NULL CHAR(32)"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"updated"`
}

func (*RNSIdentity) TableName() string {
	return "rns_identity"
}

// AddRNSIdentityTable stores Reticulum identity hashes bound to Gitea users.
func AddRNSIdentityTable(_ context.Context, x base.EngineMigration) error {
	return x.Sync(new(RNSIdentity))
}
