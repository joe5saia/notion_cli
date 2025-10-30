package notion

import (
	"context"
	"fmt"
	"net/url"
	"path"
)

// ListDataSources lists data sources under a database container.
func (c *Client) ListDataSources(ctx context.Context, databaseID string) ([]DataSource, error) {
	if databaseID == "" {
		return nil, fmt.Errorf("databaseID cannot be empty")
	}
	var resp struct {
		Results []DataSource `json:"results"`
	}
	endpoint := path.Join("databases", databaseID, "data_sources")
	if err := c.do(ctx, httpMethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// GetDataSource retrieves metadata for a single data source.
func (c *Client) GetDataSource(ctx context.Context, dataSourceID string) (DataSource, error) {
	if dataSourceID == "" {
		return DataSource{}, fmt.Errorf("dataSourceID cannot be empty")
	}
	var ds DataSource
	endpoint := path.Join("data_sources", dataSourceID)
	if err := c.do(ctx, httpMethodGet, endpoint, nil, &ds); err != nil {
		return DataSource{}, err
	}
	return ds, nil
}

// QueryDataSource executes a query against a Notion data source with pagination.
func (c *Client) QueryDataSource(
	ctx context.Context,
	dataSourceID string,
	req QueryDataSourceRequest,
) (QueryDataSourceResponse, error) {
	if dataSourceID == "" {
		return QueryDataSourceResponse{}, fmt.Errorf("dataSourceID cannot be empty")
	}
	var resp QueryDataSourceResponse
	endpoint := path.Join("data_sources", dataSourceID, "query")
	if err := c.do(ctx, httpMethodPost, endpoint, req, &resp); err != nil {
		return QueryDataSourceResponse{}, err
	}
	return resp, nil
}

// RetrievePage fetches a page by ID.
func (c *Client) RetrievePage(ctx context.Context, pageID string) (Page, error) {
	if pageID == "" {
		return Page{}, fmt.Errorf("pageID cannot be empty")
	}
	var page Page
	if err := c.do(ctx, httpMethodGet, path.Join("pages", pageID), nil, &page); err != nil {
		return Page{}, err
	}
	return page, nil
}

// UpdatePage applies changes to a page's properties or metadata.
func (c *Client) UpdatePage(ctx context.Context, pageID string, req UpdatePageRequest) (Page, error) {
	if pageID == "" {
		return Page{}, fmt.Errorf("pageID cannot be empty")
	}
	var page Page
	if err := c.do(ctx, httpMethodPatch, path.Join("pages", pageID), req, &page); err != nil {
		return Page{}, err
	}
	return page, nil
}

// AppendBlockChildren appends blocks to the specified block or page.
func (c *Client) AppendBlockChildren(ctx context.Context, blockID string, blocks []Block) error {
	if blockID == "" {
		return fmt.Errorf("blockID cannot be empty")
	}
	if len(blocks) == 0 {
		return fmt.Errorf("no blocks supplied")
	}
	req := AppendBlockChildrenRequest{Children: blocks}
	return c.do(ctx, httpMethodPatch, path.Join("blocks", blockID, "children"), req, nil)
}

// RetrieveBlockChildren fetches children blocks for a page/block.
func (c *Client) RetrieveBlockChildren(
	ctx context.Context,
	blockID string,
	startCursor string,
	pageSize int,
) (BlockChildrenResponse, error) {
	if blockID == "" {
		return BlockChildrenResponse{}, fmt.Errorf("blockID cannot be empty")
	}

	params := url.Values{}
	if startCursor != "" {
		params.Set("start_cursor", startCursor)
	}
	if pageSize > 0 {
		params.Set("page_size", fmt.Sprint(pageSize))
	}

	endpoint := path.Join("blocks", blockID, "children")
	if qs := params.Encode(); qs != "" {
		endpoint += "?" + qs
	}

	var resp BlockChildrenResponse
	if err := c.do(ctx, httpMethodGet, endpoint, nil, &resp); err != nil {
		return BlockChildrenResponse{}, err
	}
	return resp, nil
}

// RetrievePageProperty fetches a property item for large relations/rollups.
func (c *Client) RetrievePageProperty(
	ctx context.Context,
	pageID string,
	propertyID string,
	startCursor string,
) (PropertyItemResponse, error) {
	if pageID == "" || propertyID == "" {
		return PropertyItemResponse{}, fmt.Errorf("pageID and propertyID are required")
	}

	params := url.Values{}
	if startCursor != "" {
		params.Set("start_cursor", startCursor)
	}

	endpoint := path.Join("pages", pageID, "properties", propertyID)
	if qs := params.Encode(); qs != "" {
		endpoint += "?" + qs
	}

	var resp PropertyItemResponse
	if err := c.do(ctx, httpMethodGet, endpoint, nil, &resp); err != nil {
		return PropertyItemResponse{}, err
	}
	return resp, nil
}

const (
	httpMethodGet    = "GET"
	httpMethodPost   = "POST"
	httpMethodPatch  = "PATCH"
	httpMethodDelete = "DELETE"
)
