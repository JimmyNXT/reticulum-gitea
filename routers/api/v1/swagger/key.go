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

// RNSIdentity
// swagger:response RNSIdentity
type swaggerResponseRNSIdentity struct {
	// in:body
	Body api.RNSIdentity `json:"body"`
}

// RNSIdentityList
// swagger:response RNSIdentityList
type swaggerResponseRNSIdentityList struct {
	// in:body
	Body []api.RNSIdentity `json:"body"`
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
