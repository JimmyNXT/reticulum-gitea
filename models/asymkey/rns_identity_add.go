// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
)

// CheckRNSIdentityHash validates and normalizes a Reticulum identity hash.
func CheckRNSIdentityHash(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 32 {
		return "", ErrRNSInvalidIdentityHash
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", ErrRNSInvalidIdentityHash
		}
	}
	return s, nil
}

func checkIdentityHash(ctx context.Context, hash string) error {
	has, err := db.GetEngine(ctx).Where("identity_hash = ?", hash).Exist(&RNSIdentity{})
	if err != nil {
		return err
	}
	if has {
		return ErrRNSIdentityAlreadyUsed{IdentityHash: hash}
	}
	return nil
}

// AddRNSIdentity adds a Reticulum identity hash for the given user.
func AddRNSIdentity(ctx context.Context, ownerID int64, name, identityHash string) (*RNSIdentity, error) {
	hash, err := CheckRNSIdentityHash(identityHash)
	if err != nil {
		return nil, fmt.Errorf("CheckRNSIdentityHash: %w", err)
	}

	identity := &RNSIdentity{
		OwnerID:      ownerID,
		Name:         strings.TrimSpace(name),
		IdentityHash: hash,
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := checkIdentityHash(ctx, identity.IdentityHash); err != nil {
			return err
		}
		has, err := db.GetEngine(ctx).Where("owner_id = ? AND name = ?", ownerID, identity.Name).Exist(&RNSIdentity{})
		if err != nil {
			return err
		}
		if has {
			return ErrKeyNameAlreadyUsed{OwnerID: ownerID, Name: identity.Name}
		}
		return db.Insert(ctx, identity)
	})
	if err != nil {
		return nil, err
	}

	return identity, nil
}
