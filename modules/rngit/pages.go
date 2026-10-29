// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/Quad4-Software/Reticulum-Go/pkg/destination"
	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
)

// NomadNet page and file routes are served on a dedicated nomadnetwork.node
// destination on the same identity as the git destination, the aspect NomadNet
// clients browse.
const (
	pageAppName   = "nomadnetwork"
	pageAppAspect = "node"
)

// startPageNode creates the nomadnetwork.node destination and registers the
// page and file routes. Called from Start when SERVE_NOMADNET is enabled.
func (s *Server) startPageNode() error {
	dest, err := destination.New(s.identity, destination.In, destination.Single,
		pageAppName, s.node.Transport(), pageAppAspect)
	if err != nil {
		return fmt.Errorf("startPageNode: create destination: %w", err)
	}
	dest.SetDefaultAppData([]byte(s.pageNodeName()))
	for _, r := range s.pageRoutes() {
		if err := dest.RegisterRequestHandlerAny(r.path, s.wrapPageHandler(r.path, r.fn),
			destination.AllowAll, nil); err != nil {
			return fmt.Errorf("startPageNode: register %s: %w", r.path, err)
		}
	}
	s.pageDest = dest
	if err := dest.Announce(false, nil, nil); err != nil {
		log.Warn("rngit: page announce failed: %v", err)
	}
	log.Info("rngit: serving repository pages over NomadNet, destination=%s", s.PageDestinationHash())
	return nil
}

// pageNodeName is the display name the page destination announces.
func (s *Server) pageNodeName() string {
	if setting.Rngit.NomadNetName != "" {
		return setting.Rngit.NomadNetName
	}
	return setting.AppName
}

// PageDestinationHash returns the hex destination hash of the NomadNet page
// destination, or "" when pages are not served.
func (s *Server) PageDestinationHash() string {
	if s.pageDest == nil {
		return ""
	}
	return hex.EncodeToString(s.pageDest.GetHash())
}

// announcePages announces the page destination. Called from the announce loop.
func (s *Server) announcePages() {
	if s.pageDest == nil {
		return
	}
	if err := s.pageDest.Announce(false, nil, nil); err != nil {
		log.Warn("rngit: page announce failed: %v", err)
	}
}

// pageRoute pairs a request path with its handler.
type pageRoute struct {
	path string
	fn   destination.ResponseGeneratorFunc
}

// pageRoutes maps request paths to handlers on the page destination.
func (s *Server) pageRoutes() []pageRoute {
	return []pageRoute{
		{pagePathIndex, s.serveFrontPage},
		{pagePathGroup, s.serveGroupPage},
		{pagePathRepo, s.serveRepoPage},
		{pagePathTree, s.serveTreePage},
		{pagePathBlob, s.serveBlobPage},
		{pagePathCommits, s.serveCommitsPage},
		{pagePathCommit, s.serveCommitPage},
		{pagePathRefs, s.serveRefsPage},
		{pagePathStats, s.serveStatsPage},
		{pagePathReleases, s.serveReleasesPage},
		{pagePathRelease, s.serveReleasePage},
		{pagePathWork, s.serveWorkPage},
		{pagePathWorkDoc, s.serveWorkDocPage},
		{filePathArtifact, s.serveArtifact},
		{filePathDownload, s.serveDownload},
		{pagePathMedia, s.serveMediaRequest},
	}
}

// wrapPageHandler adds panic containment so a bad request cannot take down the
// destination.
func (s *Server) wrapPageHandler(path string, fn destination.ResponseGeneratorFunc) destination.ResponseGeneratorFunc {
	return func(p string, data, reqID, linkID []byte, remote *identity.Identity, at int64) (out any) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("rngit: page handler panic on %s: %v", path, r)
				out = s.renderTemplate(pageError("Internal error while rendering page"), "", "base", time.Now())
			}
		}()
		return fn(p, data, reqID, linkID, remote, at)
	}
}

// pageVars extracts the var_* parameters a NomadNet client sends for links
// with fields.
func pageVars(data []byte) map[string]string {
	vars := map[string]string{}
	req, err := rnsgit.DecodeRequest(data)
	if err != nil {
		return vars
	}
	for k, v := range req {
		ks, ok := k.(string)
		if !ok {
			if kb, ok2 := k.([]byte); ok2 {
				ks = string(kb)
				ok = true
			}
		}
		if !ok || !strings.HasPrefix(ks, "var_") {
			continue
		}
		vars[strings.TrimPrefix(ks, "var_")] = reqStringVal(v)
	}
	return vars
}

// reqStringVal renders a decoded request value as a string.
func reqStringVal(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

// encodeMixedRequest packs a mixed string/integer keyed request dict.
func encodeMixedRequest(fields map[any]any) ([]byte, error) {
	return msgpack.Marshal(fields)
}

// varInt parses an integer var, returning 1 when absent or invalid.
func varInt(vars map[string]string, key string) int {
	if v, ok := vars[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 1
}
