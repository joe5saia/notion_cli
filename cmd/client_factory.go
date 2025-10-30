package cmd

import (
	"fmt"

	"github.com/yourorg/notionctl/internal/config"
	"github.com/yourorg/notionctl/internal/notion"
)

var clientFactory = defaultClientFactory

func defaultClientFactory(profile string) (*notion.Client, error) {
	token, notionVersion, err := config.LoadAuth(profile)
	if err != nil {
		return nil, fmt.Errorf("load auth: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("profile %q has no stored Notion token", profile)
	}
	return notion.NewClient(notion.ClientConfig{
		Token:         token,
		NotionVersion: notionVersion,
	}), nil
}

func buildClient(profile string) (*notion.Client, error) {
	return clientFactory(profile)
}
