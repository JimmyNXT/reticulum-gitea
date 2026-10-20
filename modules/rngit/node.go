// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/Quad4-Software/Reticulum-Go/pkg/destination"
	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/node"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsutil"
)

// Server serves Gitea repositories over Reticulum using the rngit protocol.
type Server struct {
	ctx      context.Context
	identity *identity.Identity
	node     *node.Node
	dest     *destination.Destination
	git      *rnsgit.GitRunner
	stop     context.CancelFunc
}

// New builds a server, loading or creating the node identity.
func New() (*Server, error) {
	idPath := setting.Rngit.IdentityPath
	if idPath == "" {
		idPath = filepath.Join(setting.AppDataPath, "rngit", "identity")
	}
	id, err := rnsgit.PrepareGitIdentity(idPath)
	if err != nil {
		return nil, fmt.Errorf("New: prepare identity: %w", err)
	}
	return &Server{identity: id, git: &rnsgit.GitRunner{Git: gitcmd.GitExecutable}}, nil
}

// Run starts the node and stops it when ctx is done. It is safe to call when
// rngit is disabled.
func Run(ctx context.Context) error {
	if !setting.Rngit.Enabled {
		return nil
	}

	s, err := New()
	if err != nil {
		return err
	}
	if err := s.Start(ctx); err != nil {
		return err
	}
	context.AfterFunc(ctx, s.Stop)
	return nil
}

// Start starts the Reticulum node and begins serving requests.
func (s *Server) Start(ctx context.Context) error {
	s.ctx = ctx
	rnsCfg, err := rnsutil.LoadConfigDir(setting.Rngit.RNSConfigDir)
	if err != nil {
		return fmt.Errorf("Start: load reticulum config: %w", err)
	}

	nd, err := node.New(rnsCfg)
	if err != nil {
		return fmt.Errorf("Start: init node: %w", err)
	}
	if err := nd.Start(); err != nil {
		return fmt.Errorf("Start: start node: %w", err)
	}
	s.node = nd

	dest, err := destination.New(s.identity, destination.In, destination.Single, rnsgit.AppName, nd.Transport(), rnsgit.Aspect)
	if err != nil {
		_ = nd.Stop()
		return fmt.Errorf("Start: create destination: %w", err)
	}
	s.dest = dest

	if err := s.registerHandlers(); err != nil {
		_ = nd.Stop()
		return err
	}

	if err := s.dest.Announce(false, nil, nil); err != nil {
		log.Warn("rngit: announce failed: %v", err)
	}

	annCtx, cancel := context.WithCancel(ctx)
	s.stop = cancel
	if setting.Rngit.AnnounceInterval > 0 {
		go s.announceLoop(annCtx, time.Duration(setting.Rngit.AnnounceInterval)*time.Minute)
	}

	log.Info("rngit: serving git repositories over Reticulum, destination=%s", s.DestinationHash())
	return nil
}

// Stop shuts down the node.
func (s *Server) Stop() {
	if s.stop != nil {
		s.stop()
	}
	if s.node != nil {
		_ = s.node.Stop()
	}
}

// DestinationHash returns the hex destination hash clients use in rns:// URLs.
func (s *Server) DestinationHash() string {
	return hex.EncodeToString(s.dest.GetHash())
}

func (s *Server) announceLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.dest.Announce(false, nil, nil); err != nil {
				log.Warn("rngit: announce failed: %v", err)
			}
		}
	}
}

func (s *Server) registerHandlers() error {
	handlers := map[string]destination.ResponseGeneratorFunc{
		rnsgit.PathList:   s.handleList,
		rnsgit.PathFetch:  s.handleFetch,
		rnsgit.PathPush:   s.handlePush,
		rnsgit.PathDelete: s.handleDelete,
	}
	for path, handler := range handlers {
		if err := s.dest.RegisterRequestHandlerAny(path, handler, destination.AllowAll, nil); err != nil {
			return fmt.Errorf("registerHandlers: %s: %w", path, err)
		}
	}
	return nil
}
