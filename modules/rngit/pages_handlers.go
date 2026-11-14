// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/typesniffer"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
)

const (
	treeEntriesPerPage = 1000
	commitsPerPage     = 50
)

// pageRepo is the resolved repository context shared by page handlers.
type pageRepo struct {
	ctx    context.Context
	viewer viewer
	model  *repo_model.Repository
	git    *git.Repository
}

// openPageRepo resolves the group/repo vars for a request, enforces the view
// permission, and opens the git repository. On failure it returns an error page
// body that leaks nothing about existence.
func (s *Server) openPageRepo(vars map[string]string, remote *identity.Identity) (*pageRepo, []byte) {
	viewer := s.pageViewer(remote)
	if viewer.blocked() {
		return nil, []byte(pageError("Sign in is required to view this node"))
	}
	owner, name, ok := parseRepoVars(vars)
	if !ok {
		return nil, []byte(pageError("Repository not found"))
	}
	model := s.repoByOwnerName(owner, name)
	if model == nil || !s.canViewAs(viewer, model) {
		return nil, []byte(pageError("Repository not found"))
	}
	gitRepo, err := git.OpenRepository(s.ctx, model)
	if err != nil {
		return nil, []byte(pageError("Repository not found"))
	}
	return &pageRepo{ctx: s.ctx, viewer: viewer, model: model, git: gitRepo}, nil
}

// resolveRef resolves the ref var (default HEAD) to a commit.
func (r *pageRepo) resolveRef(refVar string) (*git.Commit, string, bool) {
	ref := strings.TrimSpace(refVar)
	if ref == "" {
		ref = "HEAD"
	}
	commit, err := r.git.GetCommit(r.ctx, ref)
	if err != nil {
		return nil, ref, false
	}
	return commit, ref, true
}

// serveFrontPage renders the repository hub listing accessible owners.
func (s *Server) serveFrontPage(_ string, _, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	viewer := s.pageViewer(remote)
	if viewer.blocked() {
		return s.renderTemplate(pageError("Sign in is required to view this node"), "", "front", st)
	}
	repos := s.accessibleRepos(viewer)
	nav := mLinkStrong("Node", pagePathIndex, nil) + " /"
	body := func(b *strings.Builder) {
		owners := make([]string, 0, len(repos))
		for owner := range repos {
			owners = append(owners, owner)
		}
		sort.Strings(owners)
		if len(owners) == 0 {
			b.WriteString("No accessible repositories\n")
		}
		for _, owner := range owners {
			b.WriteString("`a  " + mLink(owner, pagePathGroup, map[string]string{"g": owner}) + "`a\n")
		}
	}
	return s.renderPage("front", nav, st, body)
}

// serveGroupPage renders one owner's repository list.
func (s *Server) serveGroupPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	viewer := s.pageViewer(remote)
	if viewer.blocked() {
		return s.renderTemplate(pageError("Sign in is required to view this node"), "", "group", st)
	}
	vars := pageVars(data)
	owner := strings.TrimSpace(vars["g"])
	if owner == "" {
		return s.renderTemplate(pageError("No owner specified"), "", "group", st)
	}
	repos := s.accessibleRepos(viewer)[owner]
	if len(repos) == 0 {
		return s.renderTemplate(pageError("Owner not found"), "", "group", st)
	}
	nav := mLinkStrong("Node", pagePathIndex, nil) + " / " + mEscape(owner)
	body := func(b *strings.Builder) {
		sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
		for _, repo := range repos {
			b.WriteString("`a  " + mLink(repo.Name, pagePathRepo,
				map[string]string{"g": owner, "r": repo.Name}) + "`a")
			if desc := repo.Description; desc != "" {
				b.WriteString(" - " + mEscape(desc))
			}
			b.WriteString("\n")
		}
	}
	return s.renderPage("group", nav, st, body)
}

// serveRepoPage renders the repository landing page with a navigation bar and
// the readme.
func (s *Server) serveRepoPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "repo", st)
	}
	owner, name := r.model.OwnerName, r.model.Name
	fields := map[string]string{"g": owner, "r": name}
	nav := repoNav(owner, name, name)

	body := func(b *strings.Builder) {
		b.WriteString(mHeading(2) + " " + mEscape(name) + "\n\n")
		clone := fmt.Sprintf("rns://%s/%s/%s", s.DestinationHash(), owner, name)
		b.WriteString("`=`=Clone: " + mEscape(clone) + "\n`=``\n\n")
		if r.model.Description != "" {
			b.WriteString(mEscape(r.model.Description) + "\n\n")
		}

		links := []string{
			mLink(pageIconsSet.Folder+" Files", pagePathTree, fields),
			mLink(pageIconsSet.Commits+" Commits", pagePathCommits, fields),
			mLink(pageIconsSet.Branch+" Branches", pagePathRefs, mergeFields(fields, "type", "heads")),
			mLink(pageIconsSet.Tag+" Tags", pagePathRefs, mergeFields(fields, "type", "tags")),
			mLink(pageIconsSet.Stats+" Stats", pagePathStats, fields),
		}
		if n, err := db.Count[repo_model.Release](s.ctx, repo_model.FindReleasesOptions{
			RepoID: r.model.ID, IncludeDrafts: false,
		}); err == nil && n > 0 {
			links = append(links, mLink(pageIconsSet.Package+" Releases("+fmtInt(int(n))+")", pagePathReleases, fields))
		}
		b.WriteString(strings.Join(links, "  "+pageIconsSet.Sep+"  ") + "\n\n")
		b.WriteString(mDivider() + "\n\n")

		if content, ok := readmeContent(s.ctx, r.git); ok {
			scope := pagePathBlob + "`g=" + mFieldValue(owner) +
				"|r=" + mFieldValue(name) + "|ref=HEAD|path="
			b.WriteString(newMdToMicron().formatBlock(content, scope))
		}
	}
	return s.renderPage("repo", nav, st, body)
}

// serveTreePage renders the file browser for a ref/path with pagination.
func (s *Server) serveTreePage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "tree", st)
	}
	commit, displayRef, ok := r.resolveRef(vars["ref"])
	if !ok {
		return s.renderTemplate(pageError("Unknown ref"), "", "tree", st)
	}
	dirPath := decodePathVar(vars["path"])
	owner, name := r.model.OwnerName, r.model.Name
	nav := repoNav(owner, name, name) + " / " + mLink("Files", pagePathTree,
		map[string]string{"g": owner, "r": name})
	if dirPath != "" {
		nav += " / " + mEscape(dirPath)
	}

	body := func(b *strings.Builder) {
		b.WriteString(mHeading(3) + " " + mEscape(name) + " " +
			mDim(displayRef+" ("+commit.ID.String()[:8]+")") + "\n\n")
		tree := commit.Tree()
		if dirPath != "" {
			sub, err := tree.SubTree(s.ctx, r.git, dirPath)
			if err != nil {
				b.WriteString("Cannot list this path\n")
				return
			}
			tree = sub
		}
		entries, err := tree.ListEntries(s.ctx, r.git)
		if err != nil {
			b.WriteString("Cannot list this path\n")
			return
		}
		entries.CustomSort(entriesSort)

		base := map[string]string{"g": owner, "r": name, "ref": displayRef}
		if dirPath != "" {
			parent := path.Dir(dirPath)
			if parent == "." {
				parent = ""
			}
			up := map[string]string{"g": owner, "r": name, "ref": displayRef}
			if parent != "" {
				up["path"] = parent
			}
			b.WriteString(mLink("..", pagePathTree, up) + "\n\n")
		}
		if len(entries) == 0 {
			b.WriteString("Empty\n")
			return
		}
		for _, e := range entries {
			entryPath := e.Name()
			if dirPath != "" {
				entryPath = dirPath + "/" + e.Name()
			}
			switch {
			case e.IsDir():
				b.WriteString("`a  " + pageIconsSet.Folder + " " +
					mLink(e.Name()+"/", pagePathTree, mergeFields(base, "path", entryPath)) + "`a\n")
			case e.IsSubModule():
				b.WriteString("`a  ⧉ " + mEscape(e.Name()) + " " + mDim("(submodule)") + "`a\n")
			case e.IsLink():
				b.WriteString("`a  ↳ " + mEscape(e.Name()) + " " + mDim("-> "+entryLinkTarget(s.ctx, r.git, commit, entryPath)) + "`a\n")
			default:
				b.WriteString("`a  " + pageIconsSet.File + " " +
					mLink(e.Name(), pagePathBlob, mergeFields(base, "path", entryPath)) + "  " +
					mDim(prettySize(float64(e.Blob(r.git).Size(s.ctx)))) + "`a\n")
			}
		}
	}
	return s.renderPage("tree", nav, st, body)
}

// serveBlobPage renders a single file: rendered micron/markdown when
// applicable, plain text otherwise, with a download action.
func (s *Server) serveBlobPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "blob", st)
	}
	commit, displayRef, ok := r.resolveRef(vars["ref"])
	if !ok {
		return s.renderTemplate(pageError("Unknown ref"), "", "blob", st)
	}
	filePath := decodePathVar(vars["path"])
	if filePath == "" || strings.Contains(filePath, "..") {
		return s.renderTemplate(pageError("Invalid path"), "", "blob", st)
	}
	owner, name := r.model.OwnerName, r.model.Name

	entry, err := commit.GetTreeEntryByPath(s.ctx, r.git, filePath)
	if err != nil {
		return s.renderTemplate(pageError("File not found"), "", "blob", st)
	}
	if entry.IsDir() {
		vars["path"] = filePath
		return s.serveTreePage("", repackVars(vars), nil, nil, remote, 0)
	}
	blob := entry.Blob(r.git)
	if blob == nil {
		return s.renderTemplate(pageError("File not found"), "", "blob", st)
	}
	ext := strings.ToLower(path.Ext(filePath))
	renderable := ext == ".md" || ext == ".mu"
	render := vars["raw"] != "y" && renderable

	fields := map[string]string{"g": owner, "r": name, "ref": displayRef, "path": filePath}
	nav := repoNav(owner, name, name) + " / " + mLink("Files", pagePathTree, fields)

	body := func(b *strings.Builder) {
		size := blob.Size(s.ctx)
		header := mHeading(2) + " " + mEscape(filePath) + " " +
			mDim(displayRef+" ("+commit.ID.String()[:8]+")")
		header += "\n" + mDim("Text, "+prettySize(float64(size)))
		b.WriteString(header + "\n\n")

		actions := []string{mLink("Download", filePathDownload, fields)}
		if entry.IsLink() {
			actions = append(actions, mDim("-> "+entryLinkTarget(s.ctx, r.git, commit, filePath)))
		} else if renderable {
			if render {
				actions = append(actions, mLink("View raw", pagePathBlob, mergeFields(fields, "raw", "y")))
			} else {
				actions = append(actions, mLink("View rendered", pagePathBlob, fields))
			}
		}
		b.WriteString(strings.Join(actions, "  "+pageIconsSet.Sep+"  ") + "\n\n")
		b.WriteString(mDivider() + "\n\n")

		if size > blobSizeLimit {
			b.WriteString("This file is " + prettySize(float64(size)) +
				", which exceeds the display limit of " + prettySize(blobSizeLimit) + ".\n")
			return
		}
		content, err := blob.GetBlobContent(s.ctx, blobSizeLimit)
		if err != nil {
			b.WriteString("Could not read file\n")
			return
		}
		if !typesniffer.DetectContentType([]byte(content)).IsText() {
			b.WriteString("Binary file, " + prettySize(float64(size)) + ". It cannot be displayed.\n")
			return
		}
		switch {
		case render && ext == ".mu":
			b.WriteString(content)
		case render && ext == ".md":
			dir := path.Dir(filePath)
			if dir == "." {
				dir = ""
			}
			scope := pagePathBlob + "`g=" + mFieldValue(owner) +
				"|r=" + mFieldValue(name) + "|ref=" + mFieldValue(displayRef) + "|path=" + mFieldValue(dir)
			if dir != "" {
				scope += "/"
			}
			b.WriteString(newMdToMicron().formatBlock(content, scope))
		default:
			b.WriteString(plainLiteral(content))
		}
	}
	return s.renderPage("blob", nav, st, body)
}

// serveCommitsPage renders the commit log for a ref with pagination.
func (s *Server) serveCommitsPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "commits", st)
	}
	commit, displayRef, ok := r.resolveRef(vars["ref"])
	if !ok {
		return s.renderTemplate(pageError("Unknown ref"), "", "commits", st)
	}
	owner, name := r.model.OwnerName, r.model.Name
	page := max(varInt(vars, "page"), 1)
	nav := repoNav(owner, name, name) + " / " + mLink("Commits", pagePathCommits,
		map[string]string{"g": owner, "r": name})

	body := func(b *strings.Builder) {
		b.WriteString(mHeading(3) + " Commits " + mDim(displayRef+" ("+commit.ID.String()[:8]+")") + "\n\n")
		commits, err := commit.CommitsByRange(s.ctx, r.git, page, commitsPerPage, "", "", "")
		if err != nil {
			b.WriteString("Could not list commits\n")
			return
		}
		if len(commits) == 0 {
			b.WriteString("No commits\n")
			return
		}
		for _, c := range commits {
			sha := c.ID.String()
			b.WriteString("`a  " + mLink(sha[:7], pagePathCommit, map[string]string{
				"g": owner, "r": name, "ref": displayRef, "h": sha,
			}) + "  " + mEscape(c.MessageTitle()) + "\n")
			b.WriteString("`a      " + mDim(mEscape(c.Author.Name)+" - "+formatRelativeTime(c.Author.When.Unix())+" ("+prettyDate(c.Author.When.Unix())+")") + "\n")
		}
		pf := map[string]string{"g": owner, "r": name, "ref": displayRef}
		var pageLinks []string
		if page > 1 {
			pf["page"] = fmtInt(page - 1)
			pageLinks = append(pageLinks, mLink("« Newer", pagePathCommits, pf))
		}
		if len(commits) == commitsPerPage {
			pf["page"] = fmtInt(page + 1)
			pageLinks = append(pageLinks, mLink("Older »", pagePathCommits, pf))
		}
		if len(pageLinks) > 0 {
			b.WriteString("\n" + strings.Join(pageLinks, "   "+pageIconsSet.Sep+"   ") + "\n")
		}
	}
	return s.renderPage("commits", nav, st, body)
}

// serveCommitPage renders a single commit: metadata, signature status, changed
// files and the diff.
func (s *Server) serveCommitPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "commit", st)
	}
	hash := strings.TrimSpace(vars["h"])
	if len(hash) < 7 {
		return s.renderTemplate(pageError("No commit specified"), "", "commit", st)
	}
	commit, err := r.git.GetCommit(s.ctx, hash)
	if err != nil {
		return s.renderTemplate(pageError("Commit not found"), "", "commit", st)
	}
	sha := commit.ID.String()
	owner, name := r.model.OwnerName, r.model.Name
	nav := repoNav(owner, name, name) + " / " + mLink("Commits", pagePathCommits,
		map[string]string{"g": owner, "r": name}) + " / " + mEscape(sha[:7])

	body := func(b *strings.Builder) {
		b.WriteString(mHeading(2) + " Commit " + mEscape(sha[:8]) + "\n\n")
		b.WriteString(mLink(pageIconsSet.Folder+" Browse tree at this commit", pagePathTree,
			map[string]string{"g": owner, "r": name, "ref": sha}) + "\n\n")
		b.WriteString("Author:    " + mEscape(commit.Author.Name) + " " +
			mDim("<"+mEscape(commit.Author.Email)+">") + "\n")
		b.WriteString("           " + mDim(prettyDate(commit.Author.When.Unix())) + "\n")
		for _, p := range commit.Parents {
			psha := p.String()
			b.WriteString("Parent:    " + mLink(psha[:8], pagePathCommit, map[string]string{
				"g": owner, "r": name, "ref": sha, "h": psha,
			}) + "\n")
		}
		if commit.Signature != nil && commit.Signature.Signature != "" {
			b.WriteString("\nSignature: " + mDim("present") + "\n")
		}
		if msg := commit.MessageUTF8(); msg != "" {
			b.WriteString("\n" + mEscape(firstLines(msg, 40)) + "\n")
		}
		b.WriteString("\n" + mDivider() + "\n\n")
		b.WriteString(mLink("Diff available on the web interface", pagePathCommits,
			map[string]string{"g": owner, "r": name}) + "\n")
	}
	return s.renderPage("commit", nav, st, body)
}

// serveRefsPage renders branches and tags with a type filter.
func (s *Server) serveRefsPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "refs", st)
	}
	filter := vars["type"]
	if filter != "heads" && filter != "tags" {
		filter = ""
	}
	owner, name := r.model.OwnerName, r.model.Name
	base := map[string]string{"g": owner, "r": name}
	nav := repoNav(owner, name, name) + " / " + mLink("Refs", pagePathRefs, base)

	body := func(b *strings.Builder) {
		var filters []string
		filters = append(filters, mLink("All", pagePathRefs, base))
		filters = append(filters, mLink("Branches only", pagePathRefs, mergeFields(base, "type", "heads")))
		filters = append(filters, mLink("Tags only", pagePathRefs, mergeFields(base, "type", "tags")))
		b.WriteString(strings.Join(filters, "  "+pageIconsSet.Sep+"  ") + "\n\n")

		if filter == "" || filter == "heads" {
			b.WriteString(mHeading(4) + " Branches\n\n")
			branches, _, err := r.git.GetBranchNames(r.ctx, 0, 0)
			if err != nil || len(branches) == 0 {
				b.WriteString("  No branches\n")
			}
			for _, br := range branches {
				fields := mergeFields(base, "ref", br)
				b.WriteString("`a  " + pageIconsSet.Branch + " " + mLink(br, pagePathTree, fields) + "`a\n")
			}
			b.WriteString("\n")
		}
		if filter == "" || filter == "tags" {
			b.WriteString(mHeading(4) + " Tags\n\n")
			_, total, err := r.git.GetTagInfos(r.ctx, 0, 0)
			tags, _, _ := r.git.GetTagInfos(r.ctx, 1, total)
			if err != nil || len(tags) == 0 {
				b.WriteString("  No tags\n")
			}
			for _, tag := range slices.Backward(tags) {
				fields := mergeFields(base, "ref", tag.Name)
				b.WriteString("`a  " + pageIconsSet.Tag + " " + mLink(tag.Name, pagePathTree, fields) +
					"  " + mDim(mEscape(tag.MessageTitle())) + "`a\n")
			}
		}
	}
	return s.renderPage("refs", nav, st, body)
}

// serveStatsPage renders repository statistics.
func (s *Server) serveStatsPage(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	st := time.Now()
	vars := pageVars(data)
	r, errBody := s.openPageRepo(vars, remote)
	if errBody != nil {
		return s.renderTemplate(string(errBody), "", "stats", st)
	}
	owner, name := r.model.OwnerName, r.model.Name
	nav := repoNav(owner, name, name) + " / " + mLink("Stats", pagePathStats,
		map[string]string{"g": owner, "r": name})

	body := func(b *strings.Builder) {
		b.WriteString(mHeading(3) + " Statistics\n\n")
		rows := [][2]string{
			{"Stars", fmtInt(r.model.NumStars)},
			{"Forks", fmtInt(r.model.NumForks)},
			{"Watches", fmtInt(r.model.NumWatches)},
			{"Open issues", fmtInt(r.model.NumOpenIssues)},
			{"Open pull requests", fmtInt(r.model.NumOpenPulls)},
			{"Milestones", fmtInt(r.model.NumMilestones)},
			{"Size", prettySize(float64(r.model.GitSize + r.model.LFSSize))},
		}
		for _, row := range rows {
			b.WriteString("  " + mDim(row[0]+": ") + row[1] + "\n")
		}
	}
	return s.renderPage("stats", nav, st, body)
}

// serveWorkPage and serveWorkDocPage report that work tracking is not hosted.
func (s *Server) serveWorkPage(_ string, _, _, _ []byte, _ *identity.Identity, _ int64) any {
	return s.renderTemplate(notImplemented("Work pages"), "", "work", time.Now())
}

func (s *Server) serveWorkDocPage(_ string, _, _, _ []byte, _ *identity.Identity, _ int64) any {
	return s.renderTemplate(notImplemented("Work documents"), "", "work_doc", time.Now())
}

// decodePathVar URL-decodes a path var and trims surrounding slashes.
func decodePathVar(v string) string {
	if decoded, err := url.QueryUnescape(v); err == nil {
		v = decoded
	}
	return strings.Trim(v, "/")
}

// repackVars encodes a vars map back into a request dict so a blob handler can
// delegate to the tree handler when the path is a directory.
func repackVars(vars map[string]string) []byte {
	out := map[any]any{}
	for k, v := range vars {
		out["var_"+k] = v
	}
	b, _ := encodeMixedRequest(out)
	return b
}

// entryLinkTarget returns the symlink target for an entry, or "broken".
func entryLinkTarget(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, fullPath string) string {
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, fullPath)
	if err != nil {
		return "broken"
	}
	res, err := git.EntryFollowLink(ctx, gitRepo, commit, fullPath, entry)
	if err != nil {
		if res != nil && res.SymlinkContent != "" {
			return res.SymlinkContent
		}
		return "broken"
	}
	return res.SymlinkContent
}

// plainLiteral wraps text in a micron literal block, escaping backticks.
func plainLiteral(content string) string {
	var b strings.Builder
	b.WriteString("`=\n")
	for line := range strings.Lines(content) {
		b.WriteString(mEscape(strings.TrimSuffix(line, "\n")))
		b.WriteString("\n")
	}
	b.WriteString("`=")
	return b.String()
}

// mergeFields copies a field map and sets one key.
func mergeFields(base map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(base)+1)
	maps.Copy(out, base)
	if v != "" {
		out[k] = v
	}
	return out
}

// repoNav renders the shared "Node / owner / repo" breadcrumb.
func repoNav(owner, repo, label string) string {
	return mLinkStrong("Node", pagePathIndex, nil) + " / " +
		mLink(owner, pagePathGroup, map[string]string{"g": owner}) + " / " +
		mLink(label, pagePathRepo, map[string]string{"g": owner, "r": repo})
}

// entriesSort orders tree entries the way git does: directories last.
func entriesSort(s1, s2 string) int {
	switch {
	case s1 == s2:
		return 0
	case s1 == "":
		return -1
	case s2 == "":
		return 1
	default:
		return strings.Compare(s1, s2)
	}
}

// formatRelativeTime renders a timestamp as a relative phrase.
func formatRelativeTime(ts int64) string {
	diff := max(time.Now().Unix()-ts, 0)
	plural := func(n int64, unit string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s ago", n, unit)
		}
		return fmt.Sprintf("%d %ss ago", n, unit)
	}
	switch {
	case diff < 60:
		return "just now"
	case diff < 3600:
		return plural(diff/60, "minute")
	case diff < 86400:
		return plural(diff/3600, "hour")
	case diff < 604800:
		return plural(diff/86400, "day")
	case diff < 2592000:
		return plural(diff/604800, "week")
	case diff < 31536000:
		return plural(diff/2592000, "month")
	default:
		return plural(diff/31536000, "year")
	}
}

// firstLines returns at most n lines of text.
func firstLines(s string, n int) string {
	var b strings.Builder
	i := 0
	for line := range strings.SplitSeq(s, "\n") {
		if i >= n {
			b.WriteString("...")
			break
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
		i++
	}
	return b.String()
}

// fmtInt renders an int.
func fmtInt(n int) string {
	return strconv.Itoa(n)
}
