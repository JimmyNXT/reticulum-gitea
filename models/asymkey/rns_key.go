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

// RNSKey represents a Reticulum identity key
type RNSKey struct {
	ID           int64              `xorm:"pk autoincr"`
	OwnerID      int64              `xorm:"INDEX NOT NULL"`
	Name         string             `xorm:"VARCHAR(50) NOT NULL"`
	IdentityHash string             `xorm:"INDEX UNIQUE NOT NULL CHAR(32)"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(RNSKey))
}

// FindRNSKeyOptions represents the options to find RNS keys
type FindRNSKeyOptions struct {
	db.ListOptions
	OwnerID int64
}

// ToConds converts the options to conditions
func (opts FindRNSKeyOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	return cond
}

// ToOrders converts the options to orders
func (opts FindRNSKeyOptions) ToOrders() string {
	return "id"
}

// GetRNSKeyForUserByID returns RNS key by given ID.
func GetRNSKeyForUserByID(ctx context.Context, ownerID, keyID int64) (*RNSKey, error) {
	key := new(RNSKey)
	has, err := db.GetEngine(ctx).Where("id=? AND owner_id=?", keyID, ownerID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{ID: keyID}
	}
	return key, nil
}

// DeleteRNSKey deletes RNS key information.
func DeleteRNSKey(ctx context.Context, doer *user_model.User, id int64) error {
	_, err := GetRNSKeyForUserByID(ctx, doer.ID, id)
	if err != nil {
		if IsErrKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetRNSKeyForUserByID: %w", err)
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		_, err := db.GetEngine(ctx).Where("id=? AND owner_id=?", id, doer.ID).Delete(&RNSKey{})
		return err
	}); err != nil {
		return fmt.Errorf("db.WithTx: %w", err)
	}
	return nil
}

// SearchRNSKeyByHash searches for an RNS key by hash.
func SearchRNSKeyByHash(ctx context.Context, hash string) (*RNSKey, error) {
	key := new(RNSKey)
	has, err := db.GetEngine(ctx).Where("identity_hash=?", hash).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{ID: 0}
	}
	return key, nil
}
