// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"context"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/link"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
)

// publishedReleases returns the non-draft releases of a repo, newest first.
func (s *Server) publishedReleases(repoID int64) []*repo_model.Release {
	opts := repo_model.FindReleasesOptions{
		ListOptions:   db.ListOptionsAll,
		RepoID:        repoID,
		IncludeDrafts: false,
	}
	releases, err := db.Find[repo_model.Release](s.ctx, opts)
	if err != nil {
		return nil
	}
	return releases
}

// releaseByTag returns a published release with the given tag, or nil.
func (s *Server) releaseByTag(tag string, releases []*repo_model.Release) *repo_model.Release {
	for _, rel := range releases {
		if rel.TagName == tag && !rel.IsDraft {
			return rel
		}
	}
	return nil
}

// serveReleasesPage lists published releases.
func (s *Server) serveReleasesPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "releases", st)
	}
	owner, name := r.model.OwnerName, r.model.Name
	nav := repoNav(owner, name, name) + " / " + mLink("Releases", pagePathReleases,
		map[string]string{"g": owner, "r": name})

	releases := s.publishedReleases(r.model.ID)
	body := func(b *strings.Builder) {
		b.WriteString(mHeading(3) + " Releases\n\n")
		if len(releases) == 0 {
			b.WriteString("No published releases\n")
			return
		}
		for _, rel := range releases {
			fields := map[string]string{"g": owner, "r": name, "t": rel.TagName}
			b.WriteString("`a  " + pageIconsSet.Package + " " + mLink(rel.TagName, pagePathRelease, fields))
			if len(releases) > 0 && rel.ID == releases[0].ID {
				b.WriteString(" " + mFg("Latest", "0a0"))
			}
			when := rel.PublishedUnix.AsTime().Unix()
			if when == 0 {
				when = rel.CreatedUnix.AsTime().Unix()
			}
			b.WriteString("  " + mDim(prettyTimeAgo(when)) + "`a\n")
			if rel.Title != "" && rel.Title != rel.TagName {
				b.WriteString("`a      " + mEscape(rel.Title) + "`a\n")
			}
		}
	}
	return s.renderPage("releases", nav, st, body)
}

// serveReleasePage renders a single release with notes and downloadable
// attachments.
func (s *Server) serveReleasePage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "release", st)
	}
	owner, name := r.model.OwnerName, r.model.Name
	releases := s.publishedReleases(r.model.ID)
	tag := strings.TrimSpace(vars["t"])
	if tag == "" || tag == "latest" {
		if len(releases) == 0 {
			return s.renderTemplate(pageError("No releases published"), "", "release", st)
		}
		tag = releases[0].TagName
	}
	if strings.Contains(tag, "/") || strings.Contains(tag, "..") {
		return s.renderTemplate(pageError("Invalid release"), "", "release", st)
	}
	rel := s.releaseByTag(tag, releases)
	if rel == nil {
		return s.renderTemplate(pageError("Release not found"), "", "release", st)
	}
	if err := repo_model.GetReleaseAttachments(s.ctx, rel); err != nil {
		return s.renderTemplate(pageError("Could not load release"), "", "release", st)
	}
	nav := repoNav(owner, name, name) + " / " + mLink("Releases", pagePathReleases,
		map[string]string{"g": owner, "r": name}) + " / " + mEscape(tag)

	body := func(b *strings.Builder) {
		title := rel.Title
		if title == "" {
			title = rel.TagName
		}
		b.WriteString(mHeading(2) + " " + mEscape(title) + "\n")
		when := rel.PublishedUnix.AsTime().Unix()
		if when == 0 {
			when = rel.CreatedUnix.AsTime().Unix()
		}
		b.WriteString(mDim(prettyDate(when)) + "\n\n")
		if rel.IsPrerelease {
			b.WriteString("`Fd80" + "Pre-release" + "`f\n\n")
		}
		if rel.Note != "" {
			b.WriteString(newMdToMicron().formatBlock(rel.Note, "") + "\n")
		}
		if len(rel.Attachments) > 0 {
			b.WriteString("\n" + mHeading(4) + " Downloads\n\n")
			for _, a := range rel.Attachments {
				b.WriteString("`a  " + pageIconsSet.Package + " " +
					mLink(a.Name, filePathArtifact, map[string]string{
						"g": owner, "r": name, "t": tag, "a": a.Name,
					}) + "  " + mDim(prettySize(float64(a.Size))) + "`a\n")
			}
		}
	}
	return s.renderPage("release", nav, st, body)
}

// serveArtifact streams a release attachment for published releases only.
func (s *Server) serveArtifact(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return nil
	}
	tag := vars["t"]
	if decoded, err := url.QueryUnescape(tag); err == nil {
		tag = decoded
	}
	artifactName := vars["a"]
	if decoded, err := url.QueryUnescape(artifactName); err == nil {
		artifactName = decoded
	}
	if tag == "" || artifactName == "" || strings.Contains(artifactName, "/") ||
		strings.Contains(tag, "/") || strings.Contains(tag, "..") {
		return nil
	}
	releases := s.publishedReleases(r.model.ID)
	if tag == "latest" {
		if len(releases) == 0 {
			return nil
		}
		tag = releases[0].TagName
	}
	rel := s.releaseByTag(tag, releases)
	if rel == nil {
		return nil
	}
	if err := repo_model.GetReleaseAttachments(s.ctx, rel); err != nil {
		return nil
	}
	for _, a := range rel.Attachments {
		if a.Name != artifactName || path.Base(a.Name) != a.Name {
			continue
		}
		fr, err := storage.Attachments.Open(a.RelativePath())
		if err != nil {
			return nil
		}
		defer fr.Close()
		blob, err := io.ReadAll(fr)
		if err != nil {
			return nil
		}
		meta, _ := msgpack.Marshal(map[string]any{"name": []byte(a.Name)})
		return link.FileResponse{Data: blob, MetadataPacked: meta, AutoCompress: true}
	}
	return nil
}

// serveDownload streams a blob as a file response, or a micron rendering when
// fmt=mu is requested for a markdown file.
func (s *Server) serveDownload(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return nil
	}
	commit, displayRef, ok := r.resolveRef(vars["ref"])
	if !ok {
		return nil
	}
	filePath := decodePathVar(vars["path"])
	if filePath == "" || strings.Contains(filePath, "..") {
		return nil
	}
	fileFmt := vars["fmt"]
	ext := strings.ToLower(path.Ext(filePath))
	if fileFmt != "" && ext != ".md" {
		return nil
	}
	entry, err := commit.GetTreeEntryByPath(s.ctx, r.git, filePath)
	if err != nil {
		return nil
	}
	blob := entry.Blob(r.git)
	if blob == nil {
		return nil
	}
	content, err := blob.GetBlobContent(s.ctx, blobSizeLimit*8)
	if err != nil {
		return nil
	}
	name := path.Base(filePath)
	out := []byte(content)
	if fileFmt == "mu" {
		owner, repoName := r.model.OwnerName, r.model.Name
		dir := path.Dir(filePath)
		if dir == "." {
			dir = ""
		}
		urlScope := ":" + pagePathBlob + "`g=" + mFieldValue(owner) +
			"|r=" + mFieldValue(repoName) + "|ref=" + mFieldValue(displayRef) +
			"|path=" + mFieldValue(dir)
		mu := newMdToMicron().formatBlock(content, urlScope)
		if mu == "" {
			return nil
		}
		name = strings.TrimSuffix(name, ext) + ".mu"
		out = []byte(mu)
	}
	meta, _ := msgpack.Marshal(map[string]any{"name": []byte(name)})
	return link.FileResponse{Data: out, MetadataPacked: meta, AutoCompress: true}
}

// serveMediaRequest serves repository blobs referenced from rendered pages.
func (s *Server) serveMediaRequest(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	req, err := rnsgit.DecodeRequest(data)
	if err != nil || req == nil {
		return nil
	}
	if _, ok := req["key"]; !ok {
		return nil
	}
	mediaPath, _ := req["path"].(string)
	if mediaPath == "" {
		return nil
	}
	comps := strings.Split(strings.TrimLeft(strings.TrimPrefix(mediaPath, "/media"), "/"), "/")
	if len(comps) < 4 {
		return nil
	}
	owner, name, ref := comps[0], comps[1], comps[2]
	filePath := decodePathVar(strings.Join(comps[3:], "/"))

	viewer := s.pageViewer(remote)
	if viewer.blocked() {
		return nil
	}
	model := s.repoByOwnerName(owner, name)
	if model == nil || !s.canViewAs(viewer, model) {
		return nil
	}
	gitRepo, err := git.OpenRepository(s.ctx, model)
	if err != nil {
		return nil
	}
	defer gitRepo.Close()
	resolved, err := gitRepo.GetCommit(s.ctx, ref)
	if err != nil {
		return nil
	}
	entry, err := resolved.GetTreeEntryByPath(s.ctx, gitRepo, filePath)
	if err != nil {
		return nil
	}
	blob := entry.Blob(gitRepo)
	if blob == nil {
		return nil
	}
	content, err := blob.GetBlobContent(s.ctx, blobSizeLimit*8)
	if err != nil {
		return nil
	}
	meta, _ := msgpack.Marshal(map[string]any{"name": []byte(path.Base(filePath))})
	return link.FileResponse{Data: []byte(content), MetadataPacked: meta, AutoCompress: false}
}

// readmeContent loads and renders the root README of a commit as markdown.
func readmeContent(ctx context.Context, gitRepo *git.Repository) (string, bool) {
	commit, err := gitRepo.GetCommit(ctx, "HEAD")
	if err != nil {
		return "", false
	}
	tree := commit.Tree()
	entries, err := tree.ListEntries(ctx, gitRepo)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() || !util.IsReadmeFileName(e.Name()) {
			continue
		}
		content, err := e.Blob(gitRepo).GetBlobContent(ctx, blobSizeLimit)
		if err != nil {
			return "", false
		}
		return content, true
	}
	return "", false
}
