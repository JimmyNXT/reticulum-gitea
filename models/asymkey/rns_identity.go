// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(RNSIdentity))
}

// RNSIdentity represents a Reticulum identity hash associated with a user.
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

// FindRNSIdentityOptions represents the options for finding Reticulum identities.
type FindRNSIdentityOptions struct {
	db.ListOptions
	OwnerID int64
}

func (opts FindRNSIdentityOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	return cond
}

func (opts FindRNSIdentityOptions) ToOrders() string {
	return "id"
}

// GetRNSIdentityForUserByID returns the Reticulum identity matching the given id for the given owner.
func GetRNSIdentityForUserByID(ctx context.Context, ownerID, id int64) (*RNSIdentity, error) {
	identity := new(RNSIdentity)
	has, err := db.GetEngine(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Get(identity)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrKeyNotExist{ID: id}
	}
	return identity, nil
}

// DeleteRNSIdentity removes a Reticulum identity owned by the given user.
func DeleteRNSIdentity(ctx context.Context, doer *user_model.User, id int64) error {
	_, err := GetRNSIdentityForUserByID(ctx, doer.ID, id)
	if err != nil {
		if IsErrKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetRNSIdentityForUserByID: %w", err)
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		_, err := db.GetEngine(ctx).Where("id = ? AND owner_id = ?", id, doer.ID).Delete(&RNSIdentity{})
		return err
	})
	if err != nil {
		return fmt.Errorf("db.WithTx: %w", err)
	}

	return nil
}

// SearchRNSIdentityByHash returns the Reticulum identity matching the given hash.
func SearchRNSIdentityByHash(ctx context.Context, hash string) (*RNSIdentity, error) {
	identity := new(RNSIdentity)
	has, err := db.GetEngine(ctx).Where("identity_hash = ?", hash).Get(identity)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrKeyNotExist{ID: 0}
	}
	return identity, nil
}
