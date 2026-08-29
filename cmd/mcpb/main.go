// Command mcpb packages an MCP server binary into a .mcpb bundle and
// generates server.json for publishing to the MCP Registry
// (registry.modelcontextprotocol.io). It is used by the release workflow
// (see .github/workflows/release.yml) to keep packaging logic deterministic
// and free of shell/jq quoting pitfalls.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const schemaURL = "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "pack":
		err = runPack(os.Args[2:])
	case "serverjson":
		err = runServerJSON(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "mcpb: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mcpb:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mcpb — MCP Registry packaging helper

Usage:
  mcpb pack <flags>        Build a .mcpb bundle (zip: manifest.json + server/<binary>)
                           plus its .sha256 checksum file
  mcpb serverjson <flags>  Generate server.json from a directory of .mcpb bundles

Run "mcpb <command> -h" for command flags.
`)
}

// ---------------------------------------------------------------------------
// pack
// ---------------------------------------------------------------------------

type manifest struct {
	ManifestVersion string         `json:"manifest_version"`
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name,omitempty"`
	Version         string         `json:"version"`
	Description     string         `json:"description,omitempty"`
	Author          manifestAuthor `json:"author"`
	Server          manifestServer `json:"server"`
	Compatibility   manifestCompat `json:"compatibility,omitempty"`
}

type manifestAuthor struct {
	Name string `json:"name"`
}

type manifestServer struct {
	Type       string         `json:"type"`
	EntryPoint string         `json:"entry_point"`
	MCPConfig  manifestConfig `json:"mcp_config"`
}

type manifestConfig struct {
	Command string         `json:"command"`
	Args    []string       `json:"args"`
	Env     map[string]any `json:"env"`
}

type manifestCompat struct {
	Platforms []string `json:"platforms"`
}

func runPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	var (
		binary      string
		name        string
		display     string
		version     string
		description string
		author      string
		targetOS    string
		arch        string
		out         string
	)
	fs.StringVar(&binary, "binary", "", "path to the platform binary to bundle")
	fs.StringVar(&name, "name", "", "machine-readable name (used in manifest and bundle filename)")
	fs.StringVar(&display, "display", "", "human-readable display name")
	fs.StringVar(&version, "version", "", "version (leading 'v' is stripped)")
	fs.StringVar(&description, "description", "", "short description")
	fs.StringVar(&author, "author", "", "author name")
	fs.StringVar(&targetOS, "os", "", "target OS: linux, windows or darwin")
	fs.StringVar(&arch, "arch", "", "target arch: amd64 or arm64")
	fs.StringVar(&out, "out", ".", "output directory")
	fs.Parse(args)

	if binary == "" || name == "" || version == "" || targetOS == "" || arch == "" {
		return fmt.Errorf("pack requires --binary, --name, --version, --os and --arch")
	}
	platform, err := registryPlatform(targetOS)
	if err != nil {
		return err
	}
	ext := ""
	if targetOS == "windows" {
		ext = ".exe"
	}
	entry := "server/" + name + ext

	m := manifest{
		ManifestVersion: "0.3",
		Name:            name,
		DisplayName:     display,
		Version:         strings.TrimPrefix(version, "v"),
		Description:     description,
		Author:          manifestAuthor{Name: author},
		Server: manifestServer{
			Type:       "binary",
			EntryPoint: entry,
			MCPConfig: manifestConfig{
				Command: "${__dirname}/" + entry,
				Args:    []string{},
				Env:     map[string]any{},
			},
		},
		Compatibility: manifestCompat{Platforms: []string{platform}},
	}

	bundleName := fmt.Sprintf("%s-%s-%s.mcpb", name, targetOS, arch)
	if err := packBundle(binary, filepath.Join(out, bundleName), m); err != nil {
		return err
	}
	fmt.Printf("packed %s (platform %s)\n", bundleName, platform)
	return nil
}

func packBundle(binaryPath, bundlePath string, m manifest) error {
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read binary: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(bundlePath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(bundlePath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(mw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	sw, err := zw.Create(m.Server.EntryPoint)
	if err != nil {
		return err
	}
	if _, err := sw.Write(data); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	sum, err := sha256File(bundlePath)
	if err != nil {
		return err
	}
	shaPath := bundlePath + ".sha256"
	if err := os.WriteFile(shaPath, []byte(sum+"  "+filepath.Base(bundlePath)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// serverjson
// ---------------------------------------------------------------------------

type serverJSON struct {
	Schema      string     `json:"$schema"`
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description"`
	Version     string     `json:"version"`
	Repository  repository `json:"repository"`
	Packages    []pkgEntry `json:"packages"`
}

type repository struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type pkgEntry struct {
	RegistryType string    `json:"registryType"`
	Identifier   string    `json:"identifier"`
	FileSHA256   string    `json:"fileSha256"`
	Transport    transport `json:"transport"`
}

type transport struct {
	Type string `json:"type"`
}

func runServerJSON(args []string) error {
	fs := flag.NewFlagSet("serverjson", flag.ExitOnError)
	var (
		dir         string
		name        string
		title       string
		description string
		version     string
		repoURL     string
		baseTag     string
		out         string
	)
	fs.StringVar(&dir, "dir", ".", "directory containing *.mcpb and *.mcpb.sha256")
	fs.StringVar(&name, "name", "", "registry name, e.g. io.github.<owner>/<repo>")
	fs.StringVar(&title, "title", "", "human-readable title")
	fs.StringVar(&description, "description", "", "one-sentence description (max 100 chars)")
	fs.StringVar(&version, "version", "", "version (leading 'v' is stripped)")
	fs.StringVar(&repoURL, "repo-url", "", "repository URL, e.g. https://github.com/<owner>/<repo>")
	fs.StringVar(&baseTag, "base-tag", "", "release tag the .mcpb assets were uploaded to, e.g. v1.0.0")
	fs.StringVar(&out, "out", "", "output file (default: stdout)")
	fs.Parse(args)

	if name == "" || description == "" || version == "" || repoURL == "" || baseTag == "" {
		return fmt.Errorf("serverjson requires --name, --description, --version, --repo-url and --base-tag")
	}
	if len(description) > 100 {
		return fmt.Errorf("description is %d chars, registry hard limit is 100", len(description))
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.mcpb"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no *.mcpb files found in %s", dir)
	}
	sort.Strings(matches)

	baseURL := strings.TrimSuffix(repoURL, "/")
	packages := make([]pkgEntry, 0, len(matches))
	for _, m := range matches {
		sha, err := readSHA256(m + ".sha256")
		if err != nil {
			return err
		}
		packages = append(packages, pkgEntry{
			RegistryType: "mcpb",
			Identifier:   fmt.Sprintf("%s/releases/download/%s/%s", baseURL, baseTag, filepath.Base(m)),
			FileSHA256:   sha,
			Transport:    transport{Type: "stdio"},
		})
	}

	sj := serverJSON{
		Schema:      schemaURL,
		Name:        name,
		Title:       title,
		Description: description,
		Version:     strings.TrimPrefix(version, "v"),
		Repository:  repository{URL: repoURL, Source: repoSource(repoURL)},
		Packages:    packages,
	}
	data, err := json.MarshalIndent(sj, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if out == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// registryPlatform maps a Go GOOS value to the platform name used by the MCP
// Registry (mcpb compatibility.platforms).
func registryPlatform(goos string) (string, error) {
	switch goos {
	case "linux", "darwin":
		return goos, nil
	case "windows":
		return "win32", nil
	default:
		return "", fmt.Errorf("unsupported os %q (want linux, windows or darwin)", goos)
	}
}

func repoSource(repoURL string) string {
	host := strings.ToLower(repoURL)
	switch {
	case strings.Contains(host, "gitlab.com"):
		return "gitlab"
	case strings.Contains(host, "github.com"):
		return "github"
	default:
		return "github"
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readSHA256 parses the first whitespace-separated field of a sha256sum-style
// file ("<hash>  <filename>").
func readSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("%s: empty checksum file", path)
	}
	return fields[0], nil
}
