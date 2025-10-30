package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/expand"
	"github.com/yourorg/notionctl/internal/notion"
	"github.com/yourorg/notionctl/internal/render"
	"github.com/yourorg/notionctl/internal/schema"
)

//nolint:govet // fieldalignment: struct keeps related CLI options grouped logically.
type dsQueryOptions struct {
	dataSourceID     string
	format           string
	filterJSON       string
	filterFile       string
	sortsJSON        string
	sortsFile        string
	startCursor      string
	filterProperties []string
	expandRelations  []string
	pageSize         int
	fetchAll         bool

	expandRefs []notion.PropertyReference
}

func newDSQueryCmd(globals *globalOptions) *cobra.Command {
	opts := &dsQueryOptions{format: formatTable}

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query a data source for rows and properties",
		RunE:  opts.run(globals),
	}

	cmd.Flags().StringVar(&opts.dataSourceID, "data-source-id", "", "Target Notion data source ID")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: json|table")
	cmd.Flags().StringVar(&opts.filterJSON, "filter", "", "Inline JSON filter payload")
	cmd.Flags().StringVar(&opts.filterFile, "filter-file", "", "Path to JSON filter payload")
	cmd.Flags().StringVar(&opts.sortsJSON, "sorts", "", "Inline JSON sorts array")
	cmd.Flags().StringVar(&opts.sortsFile, "sorts-file", "", "Path to JSON sorts array")
	cmd.Flags().StringSliceVar(
		&opts.filterProperties,
		"filter-properties",
		nil,
		"Property names to include in the response",
	)
	cmd.Flags().StringSliceVar(&opts.expandRelations, "expand", nil, "Relation property names to expand")
	cmd.Flags().StringVar(&opts.startCursor, "start-cursor", "", "Pagination cursor to resume from")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", 0, "Page size (max 100)")
	cmd.Flags().BoolVar(&opts.fetchAll, "all", false, "Fetch all result pages (may issue multiple requests)")

	return cmd
}

func (opts *dsQueryOptions) run(globals *globalOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if err := opts.validate(); err != nil {
			return err
		}

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		resp, index, err := opts.executeQuery(ctx, client)
		if err != nil {
			return err
		}

		return opts.renderResults(cmd, resp, index)
	}
}

func (opts *dsQueryOptions) buildRequest(idx *schema.Index) (notion.QueryDataSourceRequest, error) {
	opts.expandRefs = nil

	req := notion.QueryDataSourceRequest{
		PageSize:    opts.pageSize,
		StartCursor: opts.startCursor,
	}

	filter, err := opts.buildFilter(idx)
	if err != nil {
		return notion.QueryDataSourceRequest{}, err
	}
	req.Filter = filter

	sorts, err := opts.buildSorts(idx)
	if err != nil {
		return notion.QueryDataSourceRequest{}, err
	}
	req.Sorts = sorts

	filterProps, err := opts.buildFilterProperties(idx)
	if err != nil {
		return notion.QueryDataSourceRequest{}, err
	}
	req.FilterProperties = filterProps

	expand, err := opts.buildExpandMap(idx)
	if err != nil {
		return notion.QueryDataSourceRequest{}, err
	}
	req.Expand = expand

	return req, nil
}

func (opts *dsQueryOptions) buildFilter(idx *schema.Index) (any, error) {
	payload, err := loadJSONValue(opts.filterJSON, opts.filterFile)
	if err != nil {
		return nil, fmt.Errorf("load filter: %w", err)
	}
	if payload == nil {
		return nil, nil
	}
	return mapPropertyIdentifiers(payload, idx), nil
}

func (opts *dsQueryOptions) buildSorts(idx *schema.Index) ([]any, error) {
	payload, err := loadJSONValue(opts.sortsJSON, opts.sortsFile)
	if err != nil {
		return nil, fmt.Errorf("load sorts: %w", err)
	}
	if payload == nil {
		return nil, nil
	}
	sortsSlice, ok := toSlice(payload)
	if !ok {
		return nil, errors.New("sorts payload must be a JSON array")
	}
	mapped := mapPropertyIdentifiers(sortsSlice, idx)
	mappedSlice, ok := mapped.([]any)
	if !ok {
		return nil, errors.New("sorts payload must be a JSON array of objects")
	}
	return mappedSlice, nil
}

func (opts *dsQueryOptions) buildFilterProperties(idx *schema.Index) ([]string, error) {
	if len(opts.filterProperties) == 0 {
		return nil, nil
	}

	props := make([]string, 0, len(opts.filterProperties))
	for _, name := range opts.filterProperties {
		id, ok := idx.IDForName(name)
		if !ok {
			return nil, fmt.Errorf("unknown property %q", name)
		}
		props = append(props, id)
	}
	return props, nil
}

func (opts *dsQueryOptions) buildExpandMap(idx *schema.Index) (map[string]bool, error) {
	if len(opts.expandRelations) == 0 {
		return nil, nil
	}

	expand := make(map[string]bool, len(opts.expandRelations))
	refs := make([]notion.PropertyReference, 0, len(opts.expandRelations))

	for _, name := range opts.expandRelations {
		ref, ok := idx.ReferenceForName(name)
		if !ok {
			return nil, fmt.Errorf("unknown relation %q", name)
		}
		if ref.Type != relationType {
			return nil, fmt.Errorf("property %q is not a relation", name)
		}
		expand[ref.ID] = true
		refs = append(refs, ref)
	}
	opts.expandRefs = refs
	return expand, nil
}

func executeDataSourceQuery(
	ctx context.Context,
	client *notion.Client,
	dataSourceID string,
	req notion.QueryDataSourceRequest,
	fetchAll bool,
) (notion.QueryDataSourceResponse, error) {
	if !fetchAll {
		resp, err := client.QueryDataSource(ctx, dataSourceID, req)
		if err != nil {
			return notion.QueryDataSourceResponse{}, fmt.Errorf("query data source: %w", err)
		}
		return resp, nil
	}

	var all notion.QueryDataSourceResponse
	cursor := req.StartCursor
	for {
		req.StartCursor = cursor
		resp, err := client.QueryDataSource(ctx, dataSourceID, req)
		if err != nil {
			return notion.QueryDataSourceResponse{}, fmt.Errorf("query data source: %w", err)
		}
		all.Results = append(all.Results, resp.Results...)
		all.HasMore = resp.HasMore
		all.NextCursor = resp.NextCursor
		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

func (opts *dsQueryOptions) renderResults(
	cmd *cobra.Command,
	resp notion.QueryDataSourceResponse,
	index *schema.Index,
) error {
	switch opts.format {
	case formatJSON:
		if err := render.JSON(cmd.OutOrStdout(), resp); err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		return nil
	case formatTable:
		headers, rows := queryResultsTable(resp.Results, index)
		if err := render.Table(cmd.OutOrStdout(), headers, rows); err != nil {
			return fmt.Errorf("render table: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown format %q (expected json or table)", opts.format)
	}
}

func (opts *dsQueryOptions) validate() error {
	if opts.dataSourceID == "" {
		return errors.New("--data-source-id is required")
	}
	return nil
}

func (opts *dsQueryOptions) executeQuery(
	ctx context.Context,
	client *notion.Client,
) (notion.QueryDataSourceResponse, *schema.Index, error) {
	index, err := opts.resolveIndex(ctx, client)
	if err != nil {
		return notion.QueryDataSourceResponse{}, nil, err
	}

	req, err := opts.buildRequest(index)
	if err != nil {
		return notion.QueryDataSourceResponse{}, nil, err
	}

	resp, err := executeDataSourceQuery(ctx, client, opts.dataSourceID, req, opts.fetchAll)
	if err != nil {
		return notion.QueryDataSourceResponse{}, nil, err
	}

	if err := opts.expandResults(ctx, client, resp.Results); err != nil {
		return notion.QueryDataSourceResponse{}, nil, err
	}

	return resp, index, nil
}

func (opts *dsQueryOptions) resolveIndex(ctx context.Context, client *notion.Client) (*schema.Index, error) {
	ds, err := client.GetDataSource(ctx, opts.dataSourceID)
	if err != nil {
		return nil, fmt.Errorf("get data source: %w", err)
	}
	return schema.NewIndex(ds), nil
}

func (opts *dsQueryOptions) expandResults(
	ctx context.Context,
	client expand.PageFetcher,
	pages []notion.Page,
) error {
	if len(opts.expandRefs) == 0 {
		return nil
	}
	if err := expand.FirstLevel(ctx, client, pages, opts.expandRefs); err != nil {
		return fmt.Errorf("expand relations: %w", err)
	}
	return nil
}

func loadJSONValue(inline, file string) (any, error) {
	text, err := readJSONText(inline, file)
	if err != nil || text == "" {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return payload, nil
}

func readJSONText(inline, file string) (string, error) {
	if file != "" {
		data, err := os.ReadFile(file) // #nosec G304 -- reading user-supplied filter payload is intentional
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return string(data), nil
	}
	return strings.TrimSpace(inline), nil
}

func mapPropertyIdentifiers(value any, idx *schema.Index) any {
	switch v := value.(type) {
	case map[string]any:
		return mapObjectIdentifiers(v, idx)
	case []any:
		return mapSliceIdentifiers(v, idx)
	default:
		return value
	}
}

func toSlice(value any) ([]any, bool) {
	switch s := value.(type) {
	case []any:
		return s, true
	default:
		return nil, false
	}
}

func mapObjectIdentifiers(obj map[string]any, idx *schema.Index) map[string]any {
	for key, val := range obj {
		if key == "property" {
			if name, ok := val.(string); ok {
				if id, found := idx.IDForName(name); found {
					obj[key] = id
				}
			}
			continue
		}
		obj[key] = mapPropertyIdentifiers(val, idx)
	}
	return obj
}

func mapSliceIdentifiers(values []any, idx *schema.Index) []any {
	for i := range values {
		values[i] = mapPropertyIdentifiers(values[i], idx)
	}
	return values
}

func queryResultsTable(pages []notion.Page, idx *schema.Index) ([]string, [][]string) {
	propertyNames := idx.PropertyNames()
	headers := append([]string{"ID", "Last Edited"}, propertyHeaders(propertyNames, idx)...)
	rows := make([][]string, 0, len(pages))
	for _, page := range pages {
		row := []string{page.ID, page.LastEditedTime.UTC().Format(time.RFC3339)}
		for _, name := range propertyNames {
			ref, _ := idx.ReferenceForName(name)
			value := page.Properties[ref.Name]
			row = append(row, summarizeProperty(value))
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func propertyHeaders(names []string, idx *schema.Index) []string {
	headers := make([]string, 0, len(names))
	for _, name := range names {
		ref, _ := idx.ReferenceForName(name)
		headers = append(headers, fmt.Sprintf("%s (%s)", ref.Name, ref.Type))
	}
	return headers
}

func summarizeProperty(val notion.PropertyValue) string {
	if fn, ok := propertySummaryByType[val.Type]; ok {
		return fn(val)
	}
	if len(val.Raw) > 0 {
		return string(val.Raw)
	}
	return val.Type
}

type propertySummaryFunc func(notion.PropertyValue) string

var propertySummaryByType = map[string]propertySummaryFunc{}

func init() {
	propertySummaryByType["title"] = summaryTitle
	propertySummaryByType["rich_text"] = summaryRichText
	propertySummaryByType["number"] = summaryNumber
	propertySummaryByType["status"] = summaryStatus
	propertySummaryByType["select"] = summarySelect
	propertySummaryByType["multi_select"] = summaryMultiSelect
	propertySummaryByType["checkbox"] = summaryCheckbox
	propertySummaryByType["date"] = summaryDate
	propertySummaryByType["people"] = summaryPeople
	propertySummaryByType["relation"] = summaryRelation
	propertySummaryByType["url"] = summaryURL
	propertySummaryByType["email"] = summaryEmail
	propertySummaryByType["phone_number"] = summaryPhone
	propertySummaryByType["rollup"] = summaryRollup
	propertySummaryByType["unique_id"] = summaryUniqueID
}

func summaryTitle(val notion.PropertyValue) string {
	return concatRichText(val.Title)
}

func summaryRichText(val notion.PropertyValue) string {
	return concatRichText(val.RichText)
}

func summaryNumber(val notion.PropertyValue) string {
	if val.Number == nil {
		return ""
	}
	return strconv.FormatFloat(*val.Number, 'f', -1, 64)
}

func summaryStatus(val notion.PropertyValue) string {
	if val.Status == nil {
		return ""
	}
	return val.Status.Name
}

func summarySelect(val notion.PropertyValue) string {
	if val.Select == nil {
		return ""
	}
	return val.Select.Name
}

func summaryMultiSelect(val notion.PropertyValue) string {
	return joinSelects(val.MultiSelect)
}

func summaryCheckbox(val notion.PropertyValue) string {
	if val.Checkbox == nil {
		return ""
	}
	if *val.Checkbox {
		return "true"
	}
	return "false"
}

func summaryDate(val notion.PropertyValue) string {
	if val.Date == nil {
		return ""
	}
	if val.Date.End != nil && *val.Date.End != "" {
		return fmt.Sprintf("%s → %s", val.Date.Start, *val.Date.End)
	}
	return val.Date.Start
}

func summaryPeople(val notion.PropertyValue) string {
	return joinPeople(val.People)
}

func summaryRelation(val notion.PropertyValue) string {
	return summarizeRelations(val)
}

func summaryURL(val notion.PropertyValue) string {
	return stringPtr(val.URL)
}

func summaryEmail(val notion.PropertyValue) string {
	return stringPtr(val.Email)
}

func summaryPhone(val notion.PropertyValue) string {
	return stringPtr(val.Phone)
}

func summaryRollup(val notion.PropertyValue) string {
	if val.Rollup == nil {
		return ""
	}
	switch val.Rollup.Type {
	case "number":
		if val.Rollup.Number == nil {
			return ""
		}
		return strconv.FormatFloat(*val.Rollup.Number, 'f', -1, 64)
	case "array":
		segments := make([]string, 0, len(val.Rollup.Array))
		for _, item := range val.Rollup.Array {
			segments = append(segments, summarizeProperty(item))
		}
		return strings.Join(segments, ", ")
	default:
		return val.Rollup.Type
	}
}

func summaryUniqueID(val notion.PropertyValue) string {
	if val.UniqueID == nil {
		return ""
	}
	return fmt.Sprintf("%s%d", val.UniqueID.Prefix, val.UniqueID.Number)
}

func stringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func concatRichText(parts []notion.RichText) string {
	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		segments = append(segments, p.PlainText)
	}
	return strings.Join(segments, "")
}

func joinSelects(values []notion.SelectValue) string {
	if len(values) == 0 {
		return ""
	}
	segments := make([]string, 0, len(values))
	for _, v := range values {
		segments = append(segments, v.Name)
	}
	return strings.Join(segments, ", ")
}

func joinPeople(people []notion.UserReference) string {
	if len(people) == 0 {
		return ""
	}
	names := make([]string, 0, len(people))
	for _, p := range people {
		if p.Name != "" {
			names = append(names, p.Name)
		} else {
			names = append(names, p.ID)
		}
	}
	return strings.Join(names, ", ")
}

func summarizeRelations(val notion.PropertyValue) string {
	if len(val.Relation) == 0 {
		return ""
	}
	ids := make([]string, 0, len(val.Relation))
	for _, rel := range val.Relation {
		ids = append(ids, rel.ID)
	}
	return strings.Join(ids, ", ")
}
