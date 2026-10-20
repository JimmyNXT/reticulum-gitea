// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"strings"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	repo_service "gitea.dev/services/repository"
)

// snapshotRefs lists the current refs of a repository as ref->sha.
func (s *Server) snapshotRefs(repoPath string) map[string]string {
	out, err := s.git.ListRefs(repoPath)
	if err != nil {
		return nil
	}
	refs := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.HasPrefix(fields[0], "@") {
			continue
		}
		refs[fields[1]] = fields[0]
	}
	return refs
}

// postReceive reconciles Gitea state after refs changed in the repository:
// branch rows, push notifications, webhooks and Actions. It mirrors the work
// the git post-receive hook does in-process.
func (s *Server) postReceive(repo *repo_model.Repository, user *user_model.User, before map[string]string) {
	if user == nil {
		return
	}
	ctx := s.ctx
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	empty := objectFormat.EmptyObjectID().String()

	after := s.snapshotRefs(gitrepo.RepoLocalPath(repo))
	refNames := make([]string, 0, len(before)+len(after))
	seen := make(map[string]bool, len(before)+len(after))
	for ref := range before {
		seen[ref] = true
		refNames = append(refNames, ref)
	}
	for ref := range after {
		if !seen[ref] {
			refNames = append(refNames, ref)
		}
	}

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("rngit: open repository %s: %v", repo.FullName(), err)
		return
	}
	defer gitRepo.Close()

	updates := make([]*repo_module.PushUpdateOptions, 0, len(refNames))
	branchNames := make([]string, 0, len(refNames))
	commitIDs := make([]string, 0, len(refNames))
	hasBranch := false

	for _, ref := range refNames {
		oldID, newID := before[ref], after[ref]
		if oldID == newID {
			continue
		}
		refName := git.RefName(ref)
		if !refName.IsBranch() && !refName.IsTag() {
			continue
		}
		if oldID == "" {
			oldID = empty
		}
		if newID == "" {
			newID = empty
		}
		update := &repo_module.PushUpdateOptions{
			RefFullName:  refName,
			OldCommitID:  oldID,
			NewCommitID:  newID,
			PusherID:     user.ID,
			PusherName:   user.Name,
			RepoUserName: repo.OwnerName,
			RepoName:     repo.Name,
		}
		updates = append(updates, update)
		if refName.IsBranch() {
			hasBranch = true
			if update.IsDelRef() {
				if err := git_model.MarkBranchAsDeleted(ctx, repo.ID, refName.BranchName(), user.ID); err != nil {
					log.Error("rngit: mark branch %s deleted: %v", ref, err)
				}
			} else {
				branchNames = append(branchNames, refName.BranchName())
				commitIDs = append(commitIDs, newID)
			}
		}
	}

	if len(branchNames) > 0 {
		if err := repo_service.SyncBranchesToDB(ctx, repo.ID, user.ID, gitRepo, branchNames, commitIDs); err != nil {
			log.Error("rngit: sync branches to db: %v", err)
		}
	}

	if hasBranch {
		s.ensureDefaultBranch(repo, gitRepo)
	}

	if len(updates) > 0 {
		if err := repo_service.PushUpdates(updates...); err != nil {
			log.Error("rngit: push updates: %v", err)
		}
	}
}

// ensureDefaultBranch keeps repo.DefaultBranch and the git HEAD in sync when
// the default branch does not exist yet, guessing main or master otherwise.
func (s *Server) ensureDefaultBranch(repo *repo_model.Repository, gitRepo *git.Repository) {
	ctx := s.ctx
	if _, err := gitRepo.GetBranchCommitID(ctx, repo.DefaultBranch); err == nil {
		_ = git.SetDefaultBranch(ctx, repo, repo.DefaultBranch)
		return
	}
	for _, name := range []string{"main", "master"} {
		if _, err := gitRepo.GetBranchCommitID(ctx, name); err == nil {
			repo.DefaultBranch = name
			if err := repo_model.UpdateDefaultBranch(ctx, repo); err != nil {
				log.Error("rngit: update default branch: %v", err)
			}
			return
		}
	}
}
