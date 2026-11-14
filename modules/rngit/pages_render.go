// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"gitea.dev/modules/setting"
)

const (
	micronFgReset = "`f"
	micronBgReset = "`b"

	// pageWidth is the assumed NomadNet client width used for wrapping.
	pageWidth = 100

	blobSizeLimit = 256 * 1024
)

// mEscape escapes text so micron does not interpret markup characters.
func mEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "`", "\\`")
}

// mEscapeLabel strips link markup characters from a link label.
func mEscapeLabel(s string) string {
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	return strings.ReplaceAll(s, "`", "")
}

// mFieldValue encodes a link field value the way NomadNet clients expect.
func mFieldValue(v string) string {
	return url.QueryEscape(v)
}

// mFields encodes link fields as `k=v|k=v.
func mFields(fields map[string]string) string {
	var b strings.Builder
	b.WriteString("`")
	first := true
	for _, k := range sortedKeys(fields) {
		if !first {
			b.WriteString("|")
		}
		first = false
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(mFieldValue(fields[k]))
	}
	return b.String()
}

// sortedKeys returns map keys in sorted order for deterministic output.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mLink builds a plain micron link: `label`:path`k=v|k=v.
func mLink(label, pagePath string, fields map[string]string) string {
	var b strings.Builder
	b.WriteString("`[")
	b.WriteString(mEscapeLabel(label))
	b.WriteString("`:")
	b.WriteString(pagePath)
	if len(fields) > 0 {
		b.WriteString(mFields(fields))
	}
	b.WriteString("]")
	return b.String()
}

// mLinkStrong builds a bold link.
func mLinkStrong(label, pagePath string, fields map[string]string) string {
	return "`!" + mLink(label, pagePath, fields) + "`!"
}

// mHeading returns the micron heading marker for level (1-6).
func mHeading(level int) string {
	return strings.Repeat(">", clamp(level, 1, 6))
}

// mDim dims text.
func mDim(s string) string { return "`F666" + s + "`f" }

// mFg wraps text in a micron foreground color.
func mFg(s, hexcolor string) string { return "`F" + hexcolor + s + micronFgReset }

// mDivider returns a horizontal divider.
func mDivider() string { return "-" }

// pageError renders a uniform error page body.
func pageError(msg string) string {
	return ">Notice\n\n" + mEscape(msg) + "\n"
}

// notImplemented renders the standard body for features Gitea does not host.
func notImplemented(feature string) string {
	return pageError(feature + " are not available on this node")
}

// pageIcons are the glyphs used in navigation bars.
type pageIcons struct {
	Sep     string
	Folder  string
	File    string
	Branch  string
	Tag     string
	Commits string
	Stats   string
	Heart   string
	Package string
	Work    string
}

var nerdIcons = pageIcons{
	Sep: "•", Folder: "\U000f0256", File: "\U000f0216",
	Branch: "\U000f062c", Tag: "\U000f04fc", Commits: "\U000f02da",
	Stats: "\U000f0201", Heart: "\U000f02d1", Package: "\U000f03d7",
	Work: "\U000f1323",
}

var pageIconsSet = nerdIcons

// prettySize renders a byte count in binary units.
func prettySize(num float64) string {
	suffix := "B"
	for _, s := range []string{"KB", "MB", "GB", "TB", "PB"} {
		if num < 1024 {
			break
		}
		num /= 1024
		suffix = s
	}
	if suffix == "B" {
		return fmt.Sprintf("%.0f %s", num, suffix)
	}
	return fmt.Sprintf("%.2f %s", num, suffix)
}

// prettyTime renders a duration compactly.
func prettyTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	d := time.Duration(seconds * float64(time.Second))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d years", int(d.Hours()/(24*365)))
	}
}

// prettyTimeAgo renders "x ago" for a unix timestamp.
func prettyTimeAgo(ts int64) string {
	if ts <= 0 {
		return "never"
	}
	ago := time.Since(time.Unix(ts, 0)).Seconds()
	if ago < 0 {
		ago = 0
	}
	return prettyTime(ago) + " ago"
}

// prettyDate renders an absolute timestamp.
func prettyDate(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05 UTC")
}

// formatTabs expands tab characters, matching the reference template renderer.
func formatTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "   ")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Page path constants mirrored from the NomadNet page server so clients can
// navigate between pages by the same URLs.
const (
	pagePathIndex    = "/page/index.mu"
	pagePathGroup    = "/page/group.mu"
	pagePathRepo     = "/page/repo.mu"
	pagePathTree     = "/page/tree.mu"
	pagePathBlob     = "/page/blob.mu"
	pagePathCommits  = "/page/commits.mu"
	pagePathCommit   = "/page/commit.mu"
	pagePathRefs     = "/page/refs.mu"
	pagePathStats    = "/page/stats.mu"
	pagePathReleases = "/page/releases.mu"
	pagePathRelease  = "/page/release.mu"
	pagePathWork     = "/page/work.mu"
	pagePathWorkDoc  = "/page/work_doc.mu"

	filePathArtifact = "/file/artifact"
	filePathDownload = "/file/download"

	pagePathMedia = "/media"
)

// template substitution keys used by page templates.
const (
	tplPageContent = "{PAGE_CONTENT}"
	tplNodeName    = "{NODE_NAME}"
	tplVersion     = "{VERSION}"
	tplNavigation  = "{NAVIGATION}"
	tplGenTime     = "{GEN_TIME}"
)

// rngitPageVersion is shown in page footers.
var rngitPageVersion = "gitea"

// defaultTemplates are the built-in page templates. base wraps every page,
// the rest are content injection points matching the NomadNet page server.
var defaultTemplates = map[string]string{
	"base": "#!c=0\n" +
		"> {NODE_NAME}\n\n" +
		"{NAVIGATION}\n" +
		"{PAGE_CONTENT}\n" +
		"<\n-\n" +
		"`a`F666`[Served by rngit-gitea {VERSION}`:/page/index.mu] - {GEN_TIME}`f",
	"front":    "> Repositories\n\n{PAGE_CONTENT}",
	"group":    "{PAGE_CONTENT}",
	"repo":     "{PAGE_CONTENT}",
	"releases": "{PAGE_CONTENT}",
	"release":  "{PAGE_CONTENT}",
	"tree":     "{PAGE_CONTENT}",
	"blob":     "{PAGE_CONTENT}",
	"commits":  "{PAGE_CONTENT}",
	"commit":   "{PAGE_CONTENT}",
	"refs":     "{PAGE_CONTENT}",
	"stats":    "{PAGE_CONTENT}",
	"work":     "{PAGE_CONTENT}",
	"work_doc": "{PAGE_CONTENT}",
}

// renderTemplate produces the final micron page bytes. It applies the per-page
// template first, then the base template, matching the two-stage substitution
// of the reference renderer.
func (s *Server) renderTemplate(pageContent, nav, name string, st time.Time) []byte {
	nodeName := setting.Rngit.NomadNetName
	if nodeName == "" {
		nodeName = setting.AppName
	}
	content := formatTabs(pageContent)
	if tmpl := defaultTemplates[name]; tmpl != "" {
		content = strings.ReplaceAll(tmpl, tplPageContent, content)
	}
	base := defaultTemplates["base"]
	gen := "Unknown generation time"
	if !st.IsZero() {
		gen = "Generated in " + prettyTime(time.Since(st).Seconds())
	}
	out := strings.ReplaceAll(base, tplNodeName, nodeName)
	out = strings.ReplaceAll(out, tplVersion, rngitPageVersion)
	out = strings.ReplaceAll(out, tplNavigation, nav)
	out = strings.ReplaceAll(out, tplGenTime, gen)
	out = strings.ReplaceAll(out, tplPageContent, content)
	return []byte(out)
}

// renderPage is the common entry point: build content through body and emit
// the templated page.
func (s *Server) renderPage(name, nav string, st time.Time, body func(b *strings.Builder)) []byte {
	var b strings.Builder
	b.Grow(8192)
	body(&b)
	return s.renderTemplate(b.String(), nav, name, st)
}
