package notion

import (
	"encoding/json"
	"fmt"
	"time"
)

// DataSource represents a Notion data source (table) within a database container.
type DataSource struct {
	Properties  map[string]PropertyReference `json:"properties"`
	CreatedTime time.Time                    `json:"created_time"`
	LastEdited  time.Time                    `json:"last_edited_time"`
	ID          string                       `json:"id"`
	DatabaseID  string                       `json:"database_id"`
	DataSource  string                       `json:"data_source"`
	Name        string                       `json:"name"`
}

// PropertyReference captures schema metadata for a property.
type PropertyReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryDataSourceRequest mirrors the Notion query payload for data sources.
//
//nolint:govet // fieldalignment: preserve logical grouping of JSON fields for readability.
type QueryDataSourceRequest struct {
	Filter           any             `json:"filter,omitempty"`
	Sorts            []any           `json:"sorts,omitempty"`
	FilterProperties []string        `json:"filter_properties,omitempty"`
	Aggregations     map[string]any  `json:"aggregations,omitempty"`
	Expand           map[string]bool `json:"expand,omitempty"`
	StartCursor      string          `json:"start_cursor,omitempty"`
	PageSize         int             `json:"page_size,omitempty"`
}

// QueryDataSourceResponse captures paginated query results.
//
//nolint:govet // fieldalignment: minimal benefit versus semantic ordering of fields.
type QueryDataSourceResponse struct {
	Results    []Page `json:"results"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

// Page represents a Notion page (row).
type Page struct {
	Properties        map[string]PropertyValue `json:"properties"`
	ExpandedRelations map[string][]Page        `json:"-"`
	Parent            PageParent               `json:"parent"`
	Icon              *Icon                    `json:"icon,omitempty"`
	CreatedTime       time.Time                `json:"created_time"`
	LastEditedTime    time.Time                `json:"last_edited_time"`
	ID                string                   `json:"id"`
	Object            string                   `json:"object"`
	URL               string                   `json:"url"`
	Archived          bool                     `json:"archived"`
}

// PageParent captures the page's parent container information.
type PageParent struct {
	Type         string `json:"type"`
	PageID       string `json:"page_id,omitempty"`
	DatabaseID   string `json:"database_id,omitempty"`
	DataSourceID string `json:"data_source_id,omitempty"`
}

// Icon holds either emoji or file icon data.
type Icon struct {
	Emoji *string `json:"emoji,omitempty"`
	Type  string  `json:"type"`
}

// PropertyValue represents a typed page property.
//
//nolint:govet // fieldalignment: layout keeps related property projections together.
type PropertyValue struct {
	Relation       []RelationReference `json:"relation,omitempty"`
	People         []UserReference     `json:"people,omitempty"`
	MultiSelect    []SelectValue       `json:"multi_select,omitempty"`
	RichText       []RichText          `json:"rich_text,omitempty"`
	Title          []RichText          `json:"title,omitempty"`
	Files          []FileObject        `json:"files,omitempty"`
	Raw            json.RawMessage     `json:"-"`
	Rollup         *RollupValue        `json:"rollup,omitempty"`
	Status         *StatusValue        `json:"status,omitempty"`
	Select         *SelectValue        `json:"select,omitempty"`
	Date           *DateValue          `json:"date,omitempty"`
	CreatedBy      *UserReference      `json:"created_by,omitempty"`
	LastEditedBy   *UserReference      `json:"last_edited_by,omitempty"`
	Number         *float64            `json:"number,omitempty"`
	Checkbox       *bool               `json:"checkbox,omitempty"`
	URL            *string             `json:"url,omitempty"`
	Email          *string             `json:"email,omitempty"`
	Phone          *string             `json:"phone_number,omitempty"`
	CreatedTime    *time.Time          `json:"created_time,omitempty"`
	LastEditedTime *time.Time          `json:"last_edited_time,omitempty"`
	Formula        *FormulaValue       `json:"formula,omitempty"`
	UniqueID       *UniqueIDValue      `json:"unique_id,omitempty"`
	ID             string              `json:"id"`
	Type           string              `json:"type"`
}

// UnmarshalJSON keeps the original JSON while decoding known fields.
func (p *PropertyValue) UnmarshalJSON(data []byte) error {
	type alias PropertyValue
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("unmarshal property value: %w", err)
	}
	*p = PropertyValue(tmp)
	p.Raw = append(p.Raw[:0], data...)
	return nil
}

// RelationReference references a related page.
type RelationReference struct {
	ID string `json:"id"`
}

// RollupValue captures aggregated relation data.
//
//nolint:govet // fieldalignment: retain canonical order from Notion API docs.
type RollupValue struct {
	Array  []PropertyValue `json:"array,omitempty"`
	Number *float64        `json:"number,omitempty"`
	Type   string          `json:"type"`
}

// RichText is a Notion rich text object.
type RichText struct {
	Text        *Text        `json:"text,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Href        *string      `json:"href,omitempty"`
	PlainText   string       `json:"plain_text"`
	Type        string       `json:"type"`
}

// Text contains the raw textual content.
type Text struct {
	Link *struct {
		URL string `json:"url"`
	} `json:"link,omitempty"`
	Content string `json:"content"`
}

// Annotations describe styling for rich text content.
type Annotations struct {
	Color         string `json:"color"`
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
}

// StatusValue represents a status property.
type StatusValue struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// SelectValue represents a select or multi-select option.
type SelectValue struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// DateValue represents date/time spans.
type DateValue struct {
	End   *string `json:"end,omitempty"`
	Start string  `json:"start"`
}

// FileObject references an uploaded file.
type FileObject struct {
	File *struct {
		URL        string `json:"url"`
		ExpiryTime string `json:"expiry_time"`
	} `json:"file,omitempty"`
	External *struct {
		URL string `json:"url"`
	} `json:"external,omitempty"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// UserReference references a Notion user.
type UserReference struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

// FormulaValue reflects computed formula content.
type FormulaValue struct {
	Date    *DateValue `json:"date,omitempty"`
	String  *string    `json:"string,omitempty"`
	Number  *float64   `json:"number,omitempty"`
	Boolean *bool      `json:"boolean,omitempty"`
	Type    string     `json:"type"`
}

// UniqueIDValue models the unique ID property.
//
//nolint:govet // fieldalignment: struct kept compact; rearranging offers negligible benefit.
type UniqueIDValue struct {
	Number int    `json:"number"`
	Prefix string `json:"prefix"`
}

// UpdatePageRequest represents the body for PATCH /v1/pages/{page_id}.
type UpdatePageRequest struct {
	Properties map[string]any `json:"properties,omitempty"`
	Archived   *bool          `json:"archived,omitempty"`
	Icon       *Icon          `json:"icon,omitempty"`
	Cover      *FileObject    `json:"cover,omitempty"`
}

// AppendBlockChildrenRequest for PATCH /v1/blocks/{block_id}/children.
type AppendBlockChildrenRequest struct {
	Children []Block `json:"children"`
}

// Block represents a Notion block payload.
type Block struct {
	Paragraph        *ParagraphBlock `json:"paragraph,omitempty"`
	Heading1         *HeadingBlock   `json:"heading_1,omitempty"`
	Heading2         *HeadingBlock   `json:"heading_2,omitempty"`
	Heading3         *HeadingBlock   `json:"heading_3,omitempty"`
	BulletedListItem *ParagraphBlock `json:"bulleted_list_item,omitempty"`
	NumberedListItem *ParagraphBlock `json:"numbered_list_item,omitempty"`
	ToDo             *ToDoBlock      `json:"to_do,omitempty"`
	Code             *CodeBlock      `json:"code,omitempty"`
	Quote            *ParagraphBlock `json:"quote,omitempty"`
	Callout          *CalloutBlock   `json:"callout,omitempty"`
	Toggle           *ToggleBlock    `json:"toggle,omitempty"`
	Object           string          `json:"object,omitempty"`
	Type             string          `json:"type"`
}

// ParagraphBlock contains text content shared across multiple block types.
type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
	Children []Block    `json:"children,omitempty"`
}

// HeadingBlock models heading text.
//
//nolint:govet // fieldalignment: ordering reflects Notion payload structure.
type HeadingBlock struct {
	RichText     []RichText `json:"rich_text"`
	Children     []Block    `json:"children,omitempty"`
	Color        string     `json:"color,omitempty"`
	IsToggleable bool       `json:"is_toggleable,omitempty"`
}

// ToDoBlock models todo items.
//
//nolint:govet // fieldalignment: natural field grouping preferred over padding optimization.
type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Children []Block    `json:"children,omitempty"`
	Color    string     `json:"color,omitempty"`
	Checked  bool       `json:"checked"`
}

// CodeBlock models code content.
//
//nolint:govet // fieldalignment: simple struct, padding optimisation unnecessary.
type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language,omitempty"`
}

// CalloutBlock models callout content.
//
//nolint:govet // fieldalignment: maintain intuitive field order matching API docs.
type CalloutBlock struct {
	RichText []RichText `json:"rich_text"`
	Children []Block    `json:"children,omitempty"`
	Icon     *Icon      `json:"icon,omitempty"`
	Color    string     `json:"color,omitempty"`
}

// ToggleBlock models toggle content.
//
//nolint:govet // fieldalignment: preserve readability of block fields.
type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
	Children []Block    `json:"children,omitempty"`
	Color    string     `json:"color,omitempty"`
}

// BlockChildrenResponse represents paginated block children.
//
//nolint:govet // fieldalignment: keep response metadata grouped with results.
type BlockChildrenResponse struct {
	Results    []Block `json:"results"`
	Object     string  `json:"object"`
	NextCursor string  `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// PropertyItemResponse represents paginated property item results (relations/rollups).
//
//nolint:govet // fieldalignment: readability takes precedence over minor padding gain.
type PropertyItemResponse struct {
	Results    []PropertyItem `json:"results"`
	Object     string         `json:"object"`
	NextCursor string         `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

// PropertyItem holds relation and rollup items.
//
//nolint:govet // fieldalignment: maintain JSON field order for clarity.
type PropertyItem struct {
	Value    json.RawMessage    `json:"-"`
	Relation *RelationReference `json:"relation,omitempty"`
	Page     *PageReference     `json:"page,omitempty"`
	Object   string             `json:"object"`
	Type     string             `json:"type"`
	PropertyItemPagination
}

// PageReference references a page in relation results.
type PageReference struct {
	ID string `json:"id"`
}

// PropertyItemPagination embeds common pagination metadata.
type PropertyItemPagination struct {
	NextURL string `json:"next_url,omitempty"`
}

// UnmarshalJSON retains the raw payload for property items.
func (p *PropertyItem) UnmarshalJSON(data []byte) error {
	type alias PropertyItem
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("unmarshal property item: %w", err)
	}
	*p = PropertyItem(tmp)
	p.Value = append(p.Value[:0], data...)
	return nil
}
