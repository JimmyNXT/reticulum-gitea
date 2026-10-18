// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "gitea.dev/modules/structs"
)

// PublicKey
// swagger:response PublicKey
type swaggerResponsePublicKey struct {
	// in:body
	Body api.PublicKey `json:"body"`
}

// PublicKeyList
// swagger:response PublicKeyList
type swaggerResponsePublicKeyList struct {
	// in:body
	Body []api.PublicKey `json:"body"`
}

// GPGKey
// swagger:response GPGKey
type swaggerResponseGPGKey struct {
	// in:body
	Body api.GPGKey `json:"body"`
}

// GPGKeyList
// swagger:response GPGKeyList
type swaggerResponseGPGKeyList struct {
	// in:body
	Body []api.GPGKey `json:"body"`
}

// RNSKey
// swagger:response RNSKey
type swaggerResponseRNSKey struct {
	// in:body
	Body api.RNSKey `json:"body"`
}

// RNSKeyList
// swagger:response RNSKeyList
type swaggerResponseRNSKeyList struct {
	// in:body
	Body []api.RNSKey `json:"body"`
}

// DeployKey
// swagger:response DeployKey
type swaggerResponseDeployKey struct {
	// in:body
	Body api.DeployKey `json:"body"`
}

// DeployKeyList
// swagger:response DeployKeyList
type swaggerResponseDeployKeyList struct {
	// in:body
	Body []api.DeployKey `json:"body"`
}
