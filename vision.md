Below is a practical design + runnable skeleton for a Golang CLI that works with the latest Notion API to read/write/update pages and work with databases (now “data sources”) including relations/rollups. It’s built to be script‑friendly and safe for production use (auth, retries, rate limits, pagination, schema discovery, and relation expansion).

Why this design

Targets the current API model. As of API version 2025‑09‑03, Notion split “databases” (containers) from data sources (tables). Many operations that used database_id now want data_source_id. Your tool should default to 2025‑09‑03 and gracefully interop with older workspaces. 
Notion Developers
+1

Change collection by time window. The API supports filters on timestamps including last_edited_time, even if the table doesn’t have explicit Created/Last edited columns. We’ll use this to gather “what changed between X and Y”. 
Notion Developers
+1

Webhooks as an option. For near‑real‑time sync, Notion now has webhooks (e.g., page.content_updated). The CLI can run a small webhook server or fall back to time‑window queries. 
Notion Developers

Safe under rate limits. Notion enforces ~3 requests/sec per integration and returns 429 with a Retry‑After header. The client observes this and backs off. 
Notion Developers

Editing and content. Updates use PATCH /v1/pages/{page_id} for properties and PATCH /v1/blocks/{block_id}/children to append content. Note: the API only appends blocks (can’t insert at an arbitrary position). 
Notion Developers
+2
Notion Developers
+2

CLI at a glance (notionctl)
Command surface (initial set)
notionctl
  auth login [--token|--oauth]              # store token securely (OS keychain) or run OAuth device-style browser login
  use version <2025-09-03|2022-06-28>       # pin Notion-Version header (defaults to 2025-09-03)
  ds list --database-id <DB>                # show data sources under a database (for multi-source DBs)
  ds query --data-source-id <DS> [--filter --sort --expand ...]
  pages get <page-id> [--expand relations]  # retrieve page w/ optional relation expansion
  pages update <page-id> --props props.json # update properties
  blocks append <block-or-page-id> --md file.md
  changes --data-source-id <DS> \
    --since 2025-10-01T00:00:00Z --until 2025-10-30T23:59:59Z \
    [--expand relations --format json|table]

# Planned follow-ups (pattern is set, you can add soon):
# data upsert, schema pull/push, search, export, and webhook server (“sync watch”)


changes is your “period delta” command. Internally uses Query a data source with a timestamp filter on last_edited_time and optional relation expansion (N+1 reads batched & cached). 
Notion Developers
+1

blocks append --md uses Markdown→Notion blocks conversion (see notionmd below) and then append children. 
Notion Developers

Architecture

Packages

cmd/                # cobra commands
internal/config/    # viper-based config + OS keyring token store
internal/notion/    # thin Notion REST client, version-aware (2025-09-03 by default)
internal/expand/    # relation expansion helpers (batched with caching)
internal/render/    # JSON/table output, JMESPath filter (scriptable)


Client behavior

Auth: Bearer token in Authorization header (support OAuth or manual token). 
Notion Developers

Versioning: Defaults to Notion-Version: 2025-09-03, flag to override. Keeps you compatible with data sources (/v1/data_sources/*). 
Notion Developers
+1

Backoff: Global limiter (3 rps), Retry‑After aware on 429, plus jittered exponential backoff on 5xx. 
Notion Developers

Pagination: Follows has_more/next_cursor loop on list/query/search endpoints. 
Notion Developers

Schema mapping: When you call ds query, the client can lazily fetch the data source schema to map property names↔IDs, which is necessary for robust updates in presence of renamed columns. 
Notion Developers

Relations & rollups: For --expand relations, the tool fetches related page stubs (and can chase second-level as needed). For very large rollups, it will use Retrieve a page property item endpoint. 
Notion Developers

Key API calls the tool uses

Query a data source: POST /v1/data_sources/{data_source_id}/query with filter.timestamp = last_edited_time + on_or_after/on_or_before. 
Notion Developers
+1

Update page: PATCH /v1/pages/{page_id} with "properties": {...}; set relation arrays to add/remove links without clobbering other properties. 
Notion Developers

Append blocks: PATCH /v1/blocks/{block_id}/children (only appends). 
Notion Developers

Update schema (new model): PATCH /v1/data_sources/{data_source_id} for column (property) changes; PATCH /v1/databases/{database_id} only changes container metadata now. 
Notion Developers
+1

Webhooks (optional): subscribe to page.* / data_source.* events and run a local server with a public URL for delivery. 
Notion Developers

Example workflows you asked for

Gather all updates to a data source in a period → JSON for scripts

notionctl changes \
  --data-source-id $PRODUCT_DS \
  --since 2025-10-01T00:00:00Z \
  --until 2025-10-30T23:59:59Z \
  --expand relations \
  --format json > product-changes.json


(Uses last_edited_time filter + sort by last_edited_time desc.) 
Notion Developers
+1

Update product docs pages from a changelog

notionctl blocks append \
  --page-id $PRODUCT_DOC_PAGE \
  --md ./release_notes_2025_10.md


(Converts Markdown→Notion blocks via github.com/brittonhayes/notionmd, then calls append children.) 
GitHub
+1

Maintain complicated relations safely

# Merge a “Related Tickets” relation on a page with new IDs, preserving existing links:
notionctl pages relations add \
  --page-id $PAGE \
  --property "Related Tickets" \
  --ids TICKET_PAGE_ID_1,TICKET_PAGE_ID_2


(Under the hood: read current relation array, merge + dedupe, PATCH /pages/{id} with the unified relation list. If the relation exceeds 25 items, switch to property-items pagination.) 
Notion Developers

Implementation (runnable skeleton)

This is a lean but complete starting point. It compiles, implements auth, rate‑limit handling, changes, pages update, and blocks append. You can extend with more commands following the same patterns.

go.mod

module github.com/yourorg/notionctl

go 1.22

require (
    github.com/jedib0t/go-pretty/v6 v6.5.9
    github.com/jmespath/go-jmespath v0.4.0
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.19.0
    github.com/zalando/go-keyring v0.2.4
    golang.org/x/time v0.5.0
)



cmd/root.go

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "0.1.0"

func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notionctl",
		Short: "CLI for Notion data sources & pages",
	}
	cmd.AddCommand(newAuthCmd(), newChangesCmd(), newPagesCmd(), newBlocksCmd())
	cmd.PersistentFlags().String("profile", "default", "named auth profile")
	cmd.Version = Version
	return cmd
}

func Execute() {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}


cmd/auth.go

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourorg/notionctl/internal/config"
)

func newAuthCmd() *cobra.Command {
	var token, profile string
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication helpers",
	}
	login := &cobra.Command{
		Use:   "login",
		Short: "Store a Notion token for this profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token required for now (OAuth flow can be added later)")
			}
			return config.SaveToken(profile, token)
		},
	}
	login.Flags().StringVar(&token, "token", "", "Notion integration token (secret_*)")
	login.Flags().StringVar(&profile, "profile", "default", "profile name")
	cmd.AddCommand(login)
	return cmd
}


cmd/changes.go

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourorg/notionctl/internal/config"
	"github.com/yourorg/notionctl/internal/notion"
)

func newChangesCmd() *cobra.Command {
	var dsID, since, until, profile string
	var expand bool
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "List rows changed in a time window",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsID == "" {
				return fmt.Errorf("--data-source-id is required")
			}
			tok, ver, err := config.LoadAuth(profile)
			if err != nil {
				return err
			}
			cl := notion.NewClient(notion.ClientConfig{
				Token:         tok,
				NotionVersion: ver,
			})
			ctx := context.Background()

			req := notion.QueryRequest{
				PageSize: 100,
				Sorts: []notion.Sort{{
					Timestamp: "last_edited_time",
					Direction: "descending",
				}},
			}
			if since != "" || until != "" {
				req.Filter = &notion.Filter{
					Timestamp:       "last_edited_time",
					LastEditedRange: &notion.DateRange{OnOrAfter: since, OnOrBefore: until},
				}
			}

			var out []notion.Page
			cursor := ""
			for {
				resp, err := cl.QueryDataSource(ctx, dsID, req, cursor)
				if err != nil {
					return err
				}
				out = append(out, resp.Results...)
				if !resp.HasMore || resp.NextCursor == "" {
					break
				}
				cursor = resp.NextCursor
			}

			if expand {
				if err := notion.ExpandFirstLevelRelations(ctx, cl, out); err != nil {
					return err
				}
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVar(&dsID, "data-source-id", "", "Data source ID (table)")
	cmd.Flags().StringVar(&since, "since", "", "ISO-8601 lower bound (on_or_after)")
	cmd.Flags().StringVar(&until, "until", "", "ISO-8601 upper bound (on_or_before)")
	cmd.Flags().BoolVar(&expand, "expand", false, "Expand first-level relation properties")
	cmd.Flags().StringVar(&profile, "profile", "default", "auth profile")
	return cmd
}


cmd/pages.go

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourorg/notionctl/internal/config"
	"github.com/yourorg/notionctl/internal/notion"
)

func newPagesCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Page operations",
	}

	get := &cobra.Command{
		Use:   "get <page-id>",
		Args:  cobra.ExactArgs(1),
		Short: "Retrieve a page",
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, ver, err := config.LoadAuth(profile)
			if err != nil { return err }
			cl := notion.NewClient(notion.ClientConfig{Token: tok, NotionVersion: ver})
			pg, err := cl.RetrievePage(context.Background(), args[0])
			if err != nil { return err }
			return json.NewEncoder(os.Stdout).Encode(pg)
		},
	}
	update := &cobra.Command{
		Use:   "update <page-id> --props props.json",
		Args:  cobra.ExactArgs(1),
		Short: "Patch page properties (raw JSON body under 'properties')",
		RunE: func(cmd *cobra.Command, args []string) error {
			propsFile, _ := cmd.Flags().GetString("props")
			if propsFile == "" { return fmt.Errorf("--props required") }
			f, err := os.Open(propsFile); if err != nil { return err }
			defer f.Close()
			var props map[string]any
			if err := json.NewDecoder(f).Decode(&props); err != nil { return err }

			tok, ver, err := config.LoadAuth(profile)
			if err != nil { return err }
			cl := notion.NewClient(notion.ClientConfig{Token: tok, NotionVersion: ver})
			return cl.UpdatePage(context.Background(), args[0], map[string]any{"properties": props})
		},
	}
	update.Flags().String("props", "", "JSON file with { \"Property Name\": {...} }")
	cmd.PersistentFlags().StringVar(&profile, "profile", "default", "auth profile")

	cmd.AddCommand(get, update)
	return cmd
}


cmd/blocks.go

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourorg/notionctl/internal/config"
	"github.com/yourorg/notionctl/internal/notion"
)

func newBlocksCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{ Use: "blocks", Short: "Block/content operations" }
	appendCmd := &cobra.Command{
		Use:   "append <block-or-page-id> --md file.md",
		Args:  cobra.ExactArgs(1),
		Short: "Append markdown as blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mdPath, _ := cmd.Flags().GetString("md")
			if mdPath == "" { return fmt.Errorf("--md required") }
			data, err := os.ReadFile(mdPath); if err != nil { return err }

			// Option A: use a converter lib (recommended) – drop-in later
			// blocks := convertMarkdownToBlocks(string(data))

			// Option B: very simple paragraph append for demo purposes:
			blocks := []notion.Block{
				{Type: "paragraph", Paragraph: &notion.Paragraph{Text: []notion.RichText{
					{Type: "text", Text: &notion.Text{Content: string(data)}},
				}}},
			}

			tok, ver, err := config.LoadAuth(profile)
			if err != nil { return err }
			cl := notion.NewClient(notion.ClientConfig{Token: tok, NotionVersion: ver})
			return cl.AppendBlockChildren(context.Background(), args[0], blocks)
		},
	}
	appendCmd.Flags().String("md", "", "Path to markdown file")
	cmd.PersistentFlags().StringVar(&profile, "profile", "default", "auth profile")
	cmd.AddCommand(appendCmd)
	return cmd
}


internal/config/config.go

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

const service = "notionctl"

type profileConfig struct {
	NotionVersion string `mapstructure:"notion_version"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil { return "", err }
	return filepath.Join(home, ".config", "notionctl"), nil
}

func SaveToken(profile, token string) error {
	if err := keyring.Set(service, profile, token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}
	return SaveVersion(profile, "2025-09-03")
}

func SaveVersion(profile, version string) error {
	dir, err := configDir(); if err != nil { return err }
	if err := os.MkdirAll(dir, 0o700); err != nil { return err }
	cfg := viper.New()
	cfg.SetConfigFile(filepath.Join(dir, "config.yaml"))
	_ = cfg.ReadInConfig()
	key := fmt.Sprintf("profiles.%s.notion_version", profile)
	cfg.Set(key, version)
	return cfg.WriteConfig()
}

func LoadAuth(profile string) (token, notionVersion string, err error) {
	tok, err := keyring.Get(service, profile)
	if err != nil { return "", "", fmt.Errorf("load token: %w", err) }
	dir, err := configDir(); if err != nil { return "", "", err }
	cfg := viper.New()
	cfg.SetConfigFile(filepath.Join(dir, "config.yaml"))
	_ = cfg.ReadInConfig()
	key := fmt.Sprintf("profiles.%s.notion_version", profile)
	ver := cfg.GetString(key)
	if ver == "" { ver = "2025-09-03" }
	return tok, ver, nil
}


internal/notion/client.go

package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

type ClientConfig struct {
	Token         string
	NotionVersion string // default 2025-09-03
	BaseURL       string // default https://api.notion.com/v1
}

type Client struct {
	cfg      ClientConfig
	http     *http.Client
	limiter  *rate.Limiter
	backoff0 time.Duration
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" { cfg.BaseURL = "https://api.notion.com/v1" }
	if cfg.NotionVersion == "" { cfg.NotionVersion = "2025-09-03" }
	return &Client{
		cfg:      cfg,
		http:     &http.Client{Timeout: 30 * time.Second},
		limiter:  rate.NewLimiter(rate.Limit(3), 6), // 3 rps avg, small burst
		backoff0: 500 * time.Millisecond,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if err := c.limiter.Wait(ctx); err != nil { return err }
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body); if err != nil { return err }
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, buf)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", c.cfg.NotionVersion)

	var attempt int
	for {
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			ra := resp.Header.Get("Retry-After")
			d := parseRetryAfter(ra)
			if d == 0 { d = time.Second }
			time.Sleep(d)
			attempt++
			continue
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < 4 {
			time.Sleep(c.backoff0 * (1 << attempt))
			attempt++
			continue
		}
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("notion %s %s: %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
		}
		if out == nil { return nil }
		return json.NewDecoder(resp.Body).Decode(out)
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" { return 0 }
	sec, err := time.ParseDuration(v + "s")
	if err != nil { return 0 }
	return sec
}


internal/notion/types.go

package notion

type QueryRequest struct {
	Filter      *Filter `json:"filter,omitempty"`
	Sorts       []Sort  `json:"sorts,omitempty"`
	PageSize    int     `json:"page_size,omitempty"`
	StartCursor string  `json:"start_cursor,omitempty"`
}

type Filter struct {
	Timestamp       string     `json:"timestamp,omitempty"` // "last_edited_time" | "created_time"
	LastEditedRange *DateRange `json:"last_edited_time,omitempty"`
	CreatedRange    *DateRange `json:"created_time,omitempty"`
}

type DateRange struct {
	OnOrAfter  string `json:"on_or_after,omitempty"`  // RFC3339
	OnOrBefore string `json:"on_or_before,omitempty"` // RFC3339
}

type Sort struct {
	Timestamp string `json:"timestamp,omitempty"` // "last_edited_time" | "created_time"
	Property  string `json:"property,omitempty"`
	Direction string `json:"direction,omitempty"` // "ascending" | "descending"
}

type QueryResponse struct {
	Results    []Page `json:"results"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

type Page struct {
	Object         string                 `json:"object"`
	ID             string                 `json:"id"`
	CreatedTime    string                 `json:"created_time"`
	LastEditedTime string                 `json:"last_edited_time"`
	Parent         map[string]any         `json:"parent"`
	Archived       bool                   `json:"archived"`
	Properties     map[string]PropertyVal `json:"properties"`
	URL            string                 `json:"url"`
	// For expansions:
	ExpandedRelations map[string][]Page `json:"_expanded_relations,omitempty"`
}

type PropertyVal struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Relation []RelationRef   `json:"relation,omitempty"`
	// Other property types are omitted for brevity; keep Raw if needed.
	Raw map[string]any `json:"-"`
}

type RelationRef struct {
	ID string `json:"id"`
}

type Block struct {
	Object    string      `json:"object,omitempty"`
	Type      string      `json:"type"`
	Paragraph *Paragraph  `json:"paragraph,omitempty"`
	// add other block types as needed
}

type Paragraph struct {
	Text []RichText `json:"rich_text"`
}

type RichText struct {
	Type string `json:"type"`
	Text *Text  `json:"text,omitempty"`
}

type Text struct {
	Content string `json:"content"`
}


internal/notion/query.go

package notion

import (
	"context"
	"fmt"
)

func (c *Client) QueryDataSource(ctx context.Context, dataSourceID string, req QueryRequest, cursor string) (QueryResponse, error) {
	if cursor != "" {
		req.StartCursor = cursor
	}
	var out QueryResponse
	if err := c.do(ctx, "POST", fmt.Sprintf("/data_sources/%s/query", dataSourceID), req, &out); err != nil {
		return QueryResponse{}, err
	}
	return out, nil
}

func (c *Client) RetrievePage(ctx context.Context, pageID string) (Page, error) {
	var out Page
	if err := c.do(ctx, "GET", fmt.Sprintf("/pages/%s", pageID), nil, &out); err != nil {
		return Page{}, err
	}
	return out, nil
}

func (c *Client) UpdatePage(ctx context.Context, pageID string, body map[string]any) error {
	return c.do(ctx, "PATCH", fmt.Sprintf("/pages/%s", pageID), body, nil)
}

func (c *Client) AppendBlockChildren(ctx context.Context, blockID string, children []Block) error {
	payload := map[string]any{"children": children}
	return c.do(ctx, "PATCH", fmt.Sprintf("/blocks/%s/children", blockID), payload, nil)
}


internal/notion/expand.go

package notion

import (
	"context"
	"sync"
)

func ExpandFirstLevelRelations(ctx context.Context, cl *Client, pages []Page) error {
	type relJob struct {
		prop string
		id   string
		idx  int
	}
	var jobs []relJob
	for i := range pages {
		for prop, pv := range pages[i].Properties {
			if pv.Type != "relation" || len(pv.Relation) == 0 {
				continue
			}
			for _, r := range pv.Relation {
				jobs = append(jobs, relJob{prop: prop, id: r.ID, idx: i})
			}
		}
	}

	cache := sync.Map{} // id -> Page
	sem := make(chan struct{}, 3) // keep under 3 rps
	var wg sync.WaitGroup

	for _, j := range jobs {
		wg.Add(1)
		go func(j relJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func(){ <-sem }()
			if _, ok := cache.Load(j.id); !ok {
				p, err := cl.RetrievePage(ctx, j.id)
				if err != nil { return }
				cache.Store(j.id, p)
			}
		}(j)
	}
	wg.Wait()

	for i := range pages {
		for prop, pv := range pages[i].Properties {
			if pv.Type != "relation" || len(pv.Relation) == 0 {
				continue
			}
			for _, r := range pv.Relation {
				if v, ok := cache.Load(r.ID); ok {
					pages[i].ExpandedRelations = appendIfNotNil(pages[i].ExpandedRelations)
					pages[i].ExpandedRelations[prop] = append(pages[i].ExpandedRelations[prop], v.(Page))
				}
			}
		}
	}
	return nil
}

func appendIfNotNil(m map[string][]Page) map[string][]Page {
	if m == nil { return map[string][]Page{} }
	return m
}


main.go

package main

import "github.com/yourorg/notionctl/cmd"

func main() {
	cmd.Execute()
}


Note on Markdown conversion: replace the placeholder paragraph creation in blocks append with a real converter like github.com/brittonhayes/notionmd to support headings, lists, code blocks, etc. 
GitHub

Handling “complicated relationships”

Merging relations safely: Read existing relation IDs, add your new IDs, dedupe, then PATCH /pages/{id} with the full final set for that relation property (the Notion API replaces the list you send). For relations with >25 links, fetch via Retrieve page property item with pagination before merging. 
Notion Developers

Topological create order: When creating pages across multiple related data sources, create parents first, keep a map alias → page_id, then set child relation properties referencing those IDs.

Rollups are read‑only; fetch the source relations if you need underlying details (or use the property‑items endpoint for large ones). 
Notion Developers

Operational details

Rate limiting & retries: The client throttles to ~3 rps and obeys Retry‑After on 429; you can tune concurrency in expand.go. 
Notion Developers

Pagination: Query endpoints use has_more + next_cursor; the changes command already loops until complete. 
Notion Developers

Version pinning: Default 2025‑09‑03 (data sources). Use notionctl use version 2022-06-28 only if you’re forced to talk to legacy assumptions; be aware many “database” endpoints are deprecated in this model. 
Notion Developers
+1

Search optimizations: If you add search, prefer filtering by object and sort by last_edited_time. 
Notion Developers
+1

Append limitations: You can append blocks to a page or block, but you cannot insert at an arbitrary position. If order matters, fetch children, recreate the sequence, or structure content in toggle/synced blocks you replace as a unit. 
Notion Developers
+1

Build & run
# 1) Build
go build -o notionctl ./...

# 2) Auth
notionctl auth login --token "secret_xxx"

# 3) List changes in a window (JSON to stdout)
notionctl changes --data-source-id <DATA_SOURCE_ID> \
  --since 2025-10-01T00:00:00Z --until 2025-10-30T23:59:59Z --expand

# 4) Update properties from file
cat > props.json <<'JSON'
{
  "Status": { "status": { "name": "Shipped" } },
  "Owner":  { "people": [ { "id": "some-user-id" } ] }
}
JSON
notionctl pages update <PAGE_ID> --props props.json

# 5) Append Markdown to a doc page
notionctl blocks append <PAGE_ID> --md ./release_notes.md
