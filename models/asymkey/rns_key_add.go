// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
)

// CheckRNSIdentityHash checks if the identity hash is valid.
// It returns the canonical hash or an error.
func CheckRNSIdentityHash(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	if len(s) != 32 {
		return "", ErrRNSInvalidIdentityHash
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return "", ErrRNSInvalidIdentityHash
		}
	}
	return s, nil
}

func checkIdentityHash(ctx context.Context, hash string) error {
	has, err := db.GetEngine(ctx).Where("identity_hash=?", hash).Exist(&RNSKey{})
	if err != nil {
		return err
	}
	if has {
		return ErrRNSIdentityAlreadyUsed{IdentityHash: hash}
	}
	return nil
}

// AddRNSKey adds a new RNS key
func AddRNSKey(ctx context.Context, ownerID int64, name, identityHash string) (*RNSKey, error) {
	hash, err := CheckRNSIdentityHash(identityHash)
	if err != nil {
		return nil, fmt.Errorf("CheckRNSIdentityHash: %w", err)
	}

	key := &RNSKey{
		OwnerID:      ownerID,
		Name:         strings.TrimSpace(name),
		IdentityHash: hash,
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := checkIdentityHash(ctx, key.IdentityHash); err != nil {
			return err
		}

		// check if this owner already has a key with the same name
		has, err := db.GetEngine(ctx).Where("owner_id = ? AND name = ?", ownerID, key.Name).Exist(&RNSKey{})
		if err != nil {
			return err
		}
		if has {
			return ErrKeyNameAlreadyUsed{OwnerID: ownerID, Name: key.Name}
		}

		if _, err := db.GetEngine(ctx).Insert(key); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return key, nil
}
