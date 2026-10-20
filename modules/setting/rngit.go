// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "gitea.dev/modules/log"

// RngitConfig configures serving git repositories over Reticulum (rngit).
type RngitConfig struct {
	Enabled bool

	// RNSConfigDir is the Reticulum config directory. Empty uses the
	// Reticulum-Go default location.
	RNSConfigDir string

	// IdentityPath is the node identity file. Empty uses
	// <APP_DATA_PATH>/rngit/identity.
	IdentityPath string

	// AnnounceInterval is the destination announce interval in minutes.
	// A negative value disables announcing.
	AnnounceInterval int
}

var Rngit = RngitConfig{
	AnnounceInterval: 360,
}

func loadRngitFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("rngit")
	if err := sec.MapTo(&Rngit); err != nil {
		log.Fatal("Failed to map [rngit] settings: %v", err)
	}
}
