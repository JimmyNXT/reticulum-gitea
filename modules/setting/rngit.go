// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "gitea.dev/modules/log"

// RngitConfig configures serving git repositories over Reticulum (rngit).
type RngitConfig struct {
	Enabled bool `ini:"ENABLED"`

	// RNSConfigDir is the Reticulum config directory. Empty uses the
	// Reticulum-Go default location.
	RNSConfigDir string `ini:"RNS_CONFIG_DIR"`

	// IdentityPath is the node identity file. Empty uses
	// <APP_DATA_PATH>/rngit/identity.
	IdentityPath string `ini:"IDENTITY_PATH"`

	// AnnounceInterval is the destination announce interval in minutes.
	// A negative value disables announcing.
	AnnounceInterval int `ini:"ANNOUNCE_INTERVAL"`

	// ServeNomadNet serves repository pages on a second
	// nomadnetwork.node destination, enforced with the same view
	// permissions as the website and git access.
	ServeNomadNet bool `ini:"SERVE_NOMADNET"`

	// NomadNetName is the display name announced by the page
	// destination. Empty uses the instance APP_NAME.
	NomadNetName string `ini:"NOMADNET_NAME"`
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
