package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/application"
	serializationCOM3D2 "github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/COM3D2"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/serialization/KCES/ct"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPToolsAndCapabilitiesResource(t *testing.T) {
	inputDirectory := t.TempDir()
	outputDirectory := t.TempDir()
	native := mcpSyntheticMenu(t)
	if err := os.WriteFile(filepath.Join(inputDirectory, "sample.menu"), native, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDirectory, "page.ct"), mcpContentTable(t, 5), 0644); err != nil {
		t.Fatal(err)
	}
	roots := application.NewRootSet()
	defer roots.Close()
	if err := roots.Add("mods", inputDirectory); err != nil {
		t.Fatal(err)
	}
	if err := roots.AddWritable("work", outputDirectory); err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		Engine: application.NewEngine(application.EngineOptions{
			MaxArchiveListingBytes: 1234, MaxArchiveEntries: 17,
		}), Roots: roots,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.MCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	detected, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.detect_file", Arguments: map[string]any{"root_id": "mods", "relative_path": "sample.menu"},
	})
	if err != nil || detected.IsError {
		t.Fatalf("detect tool: result=%+v err=%v", detected, err)
	}
	structured, ok := detected.StructuredContent.(map[string]any)
	if !ok || structured["format_id"] != "com3d2.menu" {
		t.Fatalf("detect structured content = %#v", detected.StructuredContent)
	}
	directBypass, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.detect_file", Arguments: map[string]any{"path": filepath.Join(inputDirectory, "sample.menu")},
	})
	if err != nil || !directBypass.IsError {
		t.Fatalf("restricted mode accepted a direct path: result=%+v err=%v", directBypass, err)
	}

	inspected, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.inspect_file", Arguments: map[string]any{"root_id": "mods", "relative_path": "sample.menu"},
	})
	if err != nil || inspected.IsError || len(inspected.Content) == 0 {
		t.Fatalf("inspect tool: result=%+v err=%v", inspected, err)
	}
	text, ok := inspected.Content[0].(*mcp.TextContent)
	if !ok || !json.Valid([]byte(text.Text)) {
		t.Fatalf("inspect content = %#v", inspected.Content)
	}
	inspectStructured, ok := inspected.StructuredContent.(map[string]any)
	if !ok || inspectStructured["format_id"] != "com3d2.menu" {
		t.Fatalf("inspect structured content = %#v", inspected.StructuredContent)
	}
	if _, duplicated := inspectStructured["editing_json"]; duplicated {
		t.Fatalf("inspect duplicated editing_json in structured content: %#v", inspectStructured)
	}

	readOnly, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.convert_file",
		Arguments: map[string]any{
			"root_id": "mods", "relative_path": "sample.menu", "target": "editing_json",
			"output_root_id": "mods", "output_relative_path": "out/denied.menu.json",
		},
	})
	if err != nil || !readOnly.IsError {
		t.Fatalf("read-only conversion: result=%+v err=%v", readOnly, err)
	}
	if _, err := os.Stat(filepath.Join(inputDirectory, "out", "denied.menu.json")); !os.IsNotExist(err) {
		t.Fatalf("read-only conversion created output: %v", err)
	}

	converted, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.convert_file",
		Arguments: map[string]any{
			"root_id": "mods", "relative_path": "sample.menu", "target": "editing_json",
			"output_root_id": "work", "output_relative_path": "out/sample.menu.json",
		},
	})
	if err != nil || converted.IsError {
		t.Fatalf("convert tool: result=%+v err=%v", converted, err)
	}
	written, err := os.ReadFile(filepath.Join(outputDirectory, "out", "sample.menu.json"))
	if err != nil || !json.Valid(written) {
		t.Fatalf("converted rooted file valid=%v err=%v", json.Valid(written), err)
	}

	token := ""
	var archiveNames []string
	for page := 0; page < 3; page++ {
		listed, listErr := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name: "meido.list_archive", Arguments: map[string]any{
				"root_id": "mods", "relative_path": "page.ct", "format_id": "kces.ct",
				"page_size": 2, "page_token": token,
			},
		})
		if listErr != nil || listed.IsError {
			t.Fatalf("list page %d: result=%+v err=%v", page, listed, listErr)
		}
		if len(listed.Content) != 0 {
			t.Fatalf("list page duplicated structured JSON as content: %#v", listed.Content)
		}
		pageData, ok := listed.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("list structured content = %#v", listed.StructuredContent)
		}
		entries, ok := pageData["entries"].([]any)
		if !ok {
			t.Fatalf("list entries = %#v", pageData["entries"])
		}
		for _, rawEntry := range entries {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				t.Fatalf("archive entry = %#v", rawEntry)
			}
			archiveNames = append(archiveNames, entry["name"].(string))
		}
		token, _ = pageData["next_page_token"].(string)
	}
	if len(archiveNames) != 5 || token != "" {
		t.Fatalf("archive pages = %v next=%q", archiveNames, token)
	}

	resource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://capabilities"})
	if err != nil || len(resource.Contents) != 1 || !json.Valid([]byte(resource.Contents[0].Text)) {
		t.Fatalf("capabilities resource = %+v, err=%v", resource, err)
	}
	var capabilities struct {
		FilesystemMode         string   `json:"filesystem_mode"`
		RootIDs                []string `json:"root_ids"`
		WritableRootIDs        []string `json:"writable_root_ids"`
		SchemaResourceTemplate string   `json:"schema_resource_template"`
		GuideResourceTemplate  string   `json:"guide_resource_template"`
		SkillResourceTemplate  string   `json:"skill_resource_template"`
		EditingPrompt          string   `json:"editing_prompt"`
		MaxArchiveListingBytes int64    `json:"max_archive_listing_bytes"`
		MaxArchiveEntries      int      `json:"max_archive_entries"`
		WritePolicy            struct {
			Mode                                  string   `json:"mode"`
			OutputArguments                       []string `json:"output_arguments"`
			RootConfinement                       bool     `json:"root_confinement"`
			RequiresExplicitUserPathAuthorization bool     `json:"requires_explicit_user_path_authorization"`
		} `json:"write_policy"`
		Formats []struct {
			ID                      string `json:"id"`
			HasEditingSchema        bool   `json:"has_editing_schema"`
			EditingSchemaID         string `json:"editing_schema_id"`
			EditingSchemaSHA256     string `json:"editing_schema_sha256"`
			HasFormatGuide          bool   `json:"has_format_guide"`
			FormatGuideVersion      string `json:"format_guide_version"`
			FormatGuideID           string `json:"format_guide_id"`
			FormatGuideSHA256       string `json:"format_guide_sha256"`
			FormatGuideVerification string `json:"format_guide_verification"`
		} `json:"formats"`
	}
	if err := json.Unmarshal([]byte(resource.Contents[0].Text), &capabilities); err != nil {
		t.Fatal(err)
	}
	if len(capabilities.RootIDs) != 2 || len(capabilities.WritableRootIDs) != 1 || capabilities.WritableRootIDs[0] != "work" {
		t.Fatalf("capability roots = %+v", capabilities)
	}
	if capabilities.FilesystemMode != string(FilesystemModeRestricted) {
		t.Fatalf("capability filesystem mode = %q", capabilities.FilesystemMode)
	}
	if capabilities.MaxArchiveListingBytes != 1234 || capabilities.MaxArchiveEntries != 17 {
		t.Fatalf("archive listing limits = %+v", capabilities)
	}
	if capabilities.WritePolicy.Mode != "configured_writable_roots" ||
		!slices.Equal(capabilities.WritePolicy.OutputArguments, []string{"output_root_id", "output_relative_path"}) ||
		!capabilities.WritePolicy.RootConfinement || capabilities.WritePolicy.RequiresExplicitUserPathAuthorization {
		t.Fatalf("restricted write policy = %+v", capabilities.WritePolicy)
	}
	if capabilities.SchemaResourceTemplate != "meido://schemas/{format_id}" {
		t.Fatalf("schema resource template = %q", capabilities.SchemaResourceTemplate)
	}
	if capabilities.GuideResourceTemplate != "meido://guides/{format_id}" || capabilities.SkillResourceTemplate != "meido://skills/editing/{format_id}" {
		t.Fatalf("knowledge resource templates = guide %q skill %q", capabilities.GuideResourceTemplate, capabilities.SkillResourceTemplate)
	}
	if capabilities.EditingPrompt != "meido.edit_format" {
		t.Fatalf("editing prompt = %q", capabilities.EditingPrompt)
	}
	var menuCapability, mateCapability *struct {
		ID                      string `json:"id"`
		HasEditingSchema        bool   `json:"has_editing_schema"`
		EditingSchemaID         string `json:"editing_schema_id"`
		EditingSchemaSHA256     string `json:"editing_schema_sha256"`
		HasFormatGuide          bool   `json:"has_format_guide"`
		FormatGuideVersion      string `json:"format_guide_version"`
		FormatGuideID           string `json:"format_guide_id"`
		FormatGuideSHA256       string `json:"format_guide_sha256"`
		FormatGuideVerification string `json:"format_guide_verification"`
	}
	for _, format := range capabilities.Formats {
		if format.HasEditingSchema && (!format.HasFormatGuide || format.FormatGuideVerification != "serialization_verified") {
			t.Fatalf("editing format has no reviewed guide: %+v", format)
		}
		switch format.ID {
		case "com3d2.menu":
			value := format
			menuCapability = &value
		case "com3d2.mate":
			value := format
			mateCapability = &value
		}
	}
	if menuCapability == nil || !menuCapability.HasEditingSchema || menuCapability.EditingSchemaID == "" || menuCapability.EditingSchemaSHA256 == "" {
		t.Fatalf("menu schema is not advertised: %+v", menuCapability)
	}
	if !menuCapability.HasFormatGuide || menuCapability.FormatGuideVersion == "" || menuCapability.FormatGuideID == "" || menuCapability.FormatGuideSHA256 == "" || menuCapability.FormatGuideVerification != "serialization_verified" {
		t.Fatalf("menu guide is not advertised: %+v", menuCapability)
	}
	if mateCapability == nil || !mateCapability.HasFormatGuide || mateCapability.FormatGuideVerification != "serialization_verified" {
		t.Fatalf("mate guide verification = %+v", mateCapability)
	}

	resourceTemplates, err := clientSession.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	templatesByURI := make(map[string]string)
	for _, resourceTemplate := range resourceTemplates.ResourceTemplates {
		templatesByURI[resourceTemplate.URITemplate] = resourceTemplate.MIMEType
	}
	for uri, mediaType := range map[string]string{
		"meido://schemas/{format_id}":        "application/schema+json",
		"meido://guides/{format_id}":         "application/vnd.meido.format-guide+json",
		"meido://skills/editing/{format_id}": "text/markdown",
	} {
		if templatesByURI[uri] != mediaType {
			t.Fatalf("resource template %s = %q (all=%#v)", uri, templatesByURI[uri], templatesByURI)
		}
	}

	schemaResource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://schemas/com3d2.menu"})
	if err != nil || len(schemaResource.Contents) != 1 || schemaResource.Contents[0].MIMEType != "application/schema+json" || !json.Valid([]byte(schemaResource.Contents[0].Text)) {
		t.Fatalf("schema resource = %+v, err=%v", schemaResource, err)
	}
	var schemaHeader struct {
		FormatID string `json:"x-meido-format-id"`
		Version  string `json:"x-meido-schema-version"`
	}
	if err := json.Unmarshal([]byte(schemaResource.Contents[0].Text), &schemaHeader); err != nil || schemaHeader.FormatID != "com3d2.menu" || schemaHeader.Version == "" {
		t.Fatalf("schema header = %+v err=%v", schemaHeader, err)
	}

	for _, test := range []struct {
		formatID     string
		verification string
	}{
		{formatID: "com3d2.menu", verification: "serialization_verified"},
		{formatID: "kces.dbconf", verification: "serialization_verified"},
		{formatID: "kces.dbcol", verification: "serialization_verified"},
		{formatID: "com3d2.mate", verification: "serialization_verified"},
		{formatID: "kces.nson", verification: "serialization_verified"},
	} {
		guideResource, readErr := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://guides/" + test.formatID})
		if readErr != nil || len(guideResource.Contents) != 1 || guideResource.Contents[0].MIMEType != "application/vnd.meido.format-guide+json" || !json.Valid([]byte(guideResource.Contents[0].Text)) {
			t.Fatalf("guide resource %s = %+v, err=%v", test.formatID, guideResource, readErr)
		}
		var guideHeader struct {
			FormatID           string `json:"format_id"`
			SchemaID           string `json:"schema_id"`
			FormatVerification struct {
				Level string `json:"level"`
			} `json:"format_verification"`
			Fields []json.RawMessage `json:"fields"`
		}
		if err := json.Unmarshal([]byte(guideResource.Contents[0].Text), &guideHeader); err != nil {
			t.Fatal(err)
		}
		if guideHeader.FormatID != test.formatID || guideHeader.SchemaID == "" || guideHeader.FormatVerification.Level != test.verification || len(guideHeader.Fields) == 0 {
			t.Fatalf("guide header %s = %+v", test.formatID, guideHeader)
		}
		for _, advertised := range capabilities.Formats {
			if advertised.ID == test.formatID && advertised.FormatGuideSHA256 != mcpSHA256([]byte(guideResource.Contents[0].Text)) {
				t.Fatalf("guide digest %s = %q, capability = %q", test.formatID, mcpSHA256([]byte(guideResource.Contents[0].Text)), advertised.FormatGuideSHA256)
			}
		}
	}

	skillResource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://skills/editing/com3d2.menu"})
	if err != nil || len(skillResource.Contents) != 1 || skillResource.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("skill resource = %+v, err=%v", skillResource, err)
	}
	skillText := skillResource.Contents[0].Text
	if !strings.Contains(skillText, "Format: `com3d2.menu`") || !strings.Contains(skillText, "Whole-file verification: `serialization_verified`") || !strings.Contains(skillText, "meido.validate_editing_json") ||
		!strings.Contains(skillText, "follow the write policy declared by the current") ||
		!strings.Contains(skillText, "Use `output_root_id` and `output_relative_path`") ||
		!strings.Contains(skillText, "configured writable root declared by `meido://capabilities`") ||
		strings.Contains(skillText, "Use `output_path` only") {
		t.Fatalf("skill resource content = %q", skillText)
	}

	prompts, err := clientSession.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundEditingPrompt := false
	for _, prompt := range prompts.Prompts {
		if prompt.Name == "meido.edit_format" {
			foundEditingPrompt = true
			break
		}
	}
	if !foundEditingPrompt {
		t.Fatalf("editing prompt is not listed: %+v", prompts.Prompts)
	}
	prompt, err := clientSession.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "meido.edit_format",
		Arguments: map[string]string{
			"format_id": "com3d2.menu", "objective": "Change only the displayed item name.",
			"input_root_id": "mods", "input_relative_path": "sample.menu",
			"output_root_id": "work", "output_relative_path": "out/sample.menu",
		},
	})
	if err != nil || len(prompt.Messages) != 4 {
		t.Fatalf("editing prompt = %+v, err=%v", prompt, err)
	}
	promptSkill, ok := prompt.Messages[0].Content.(*mcp.TextContent)
	if !ok || promptSkill.Text != skillText {
		t.Fatalf("restricted prompt and resource use different skills: prompt=%#v resource=%q", prompt.Messages[0].Content, skillText)
	}
	resourceURIs := make(map[string]string)
	for _, message := range prompt.Messages {
		embedded, ok := message.Content.(*mcp.EmbeddedResource)
		if !ok {
			continue
		}
		if embedded.Resource == nil {
			t.Fatal("editing prompt contains an empty embedded resource")
		}
		resourceURIs[embedded.Resource.URI] = embedded.Resource.Text
	}
	if len(resourceURIs) != 2 || !json.Valid([]byte(resourceURIs["meido://schemas/com3d2.menu"])) || !json.Valid([]byte(resourceURIs["meido://guides/com3d2.menu"])) {
		t.Fatalf("editing prompt embedded resources = %#v", resourceURIs)
	}
}

func TestMCPUnrestrictedFilesystemMode(t *testing.T) {
	directory := t.TempDir()
	menuPath := filepath.Join(directory, "sample.menu")
	archivePath := filepath.Join(directory, "page.ct")
	if err := os.WriteFile(menuPath, mcpSyntheticMenu(t), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, mcpContentTable(t, 3), 0644); err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		Engine: application.NewEngine(application.EngineOptions{}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.filesystemMode != FilesystemModeUnrestricted {
		t.Fatalf("default filesystem mode = %q", server.filesystemMode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.MCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	detected, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.detect_file", Arguments: map[string]any{"path": menuPath},
	})
	if err != nil || detected.IsError {
		t.Fatalf("direct detect: result=%+v err=%v", detected, err)
	}
	rootedBypass, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.detect_file", Arguments: map[string]any{"root_id": "unused", "relative_path": "sample.menu"},
	})
	if err != nil || !rootedBypass.IsError {
		t.Fatalf("unrestricted mode accepted rooted arguments: result=%+v err=%v", rootedBypass, err)
	}

	inspected, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.inspect_file", Arguments: map[string]any{"path": menuPath},
	})
	if err != nil || inspected.IsError || len(inspected.Content) != 1 {
		t.Fatalf("direct inspect: result=%+v err=%v", inspected, err)
	}
	if text, ok := inspected.Content[0].(*mcp.TextContent); !ok || !json.Valid([]byte(text.Text)) {
		t.Fatalf("direct inspect content = %#v", inspected.Content)
	}

	editingPath := filepath.Join(directory, "out", "sample.menu.json")
	if err := os.MkdirAll(filepath.Dir(editingPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(editingPath+".meta.json", []byte(`{"stale":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	converted, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.convert_file", Arguments: map[string]any{
			"path": menuPath, "target": "editing_json", "output_path": editingPath,
		},
	})
	if err != nil || converted.IsError {
		t.Fatalf("direct convert: result=%+v err=%v", converted, err)
	}
	convertedData, ok := converted.StructuredContent.(map[string]any)
	if !ok || convertedData["path"] != editingPath || convertedData["root_id"] != nil {
		t.Fatalf("direct conversion location = %#v", converted.StructuredContent)
	}
	if data, readErr := os.ReadFile(editingPath); readErr != nil || !json.Valid(data) {
		t.Fatalf("direct conversion output valid=%v err=%v", json.Valid(data), readErr)
	}
	if _, statErr := os.Stat(editingPath + ".meta.json"); !os.IsNotExist(statErr) {
		t.Fatalf("direct conversion retained stale sidecar: %v", statErr)
	}

	validated, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.validate_editing_json", Arguments: map[string]any{"path": editingPath, "format_id": "com3d2.menu"},
	})
	if err != nil || validated.IsError {
		t.Fatalf("direct validate: result=%+v err=%v", validated, err)
	}

	listed, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.list_archive", Arguments: map[string]any{"path": archivePath, "format_id": "kces.ct", "page_size": 2},
	})
	if err != nil || listed.IsError {
		t.Fatalf("direct list: result=%+v err=%v", listed, err)
	}
	listData, ok := listed.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("direct archive page = %#v", listed.StructuredContent)
	}
	entries, ok := listData["entries"].([]any)
	nextToken, tokenOK := listData["next_page_token"].(string)
	if !ok || len(entries) != 2 || !tokenOK || nextToken == "" || nextToken == "2" {
		t.Fatalf("direct archive page = %#v", listed.StructuredContent)
	}

	extractedPath := filepath.Join(directory, "out", "entry-01.bin")
	extracted, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "meido.extract_archive_entry", Arguments: map[string]any{
			"path": archivePath, "format_id": "kces.ct", "entry_name": "entry-01.bin", "output_path": extractedPath,
		},
	})
	if err != nil || extracted.IsError {
		t.Fatalf("direct extract: result=%+v err=%v", extracted, err)
	}
	if data, readErr := os.ReadFile(extractedPath); readErr != nil || !bytes.Equal(data, []byte{1}) {
		t.Fatalf("direct extracted data = %v err=%v", data, readErr)
	}

	resource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://capabilities"})
	if err != nil || len(resource.Contents) != 1 {
		t.Fatalf("direct capabilities = %+v err=%v", resource, err)
	}
	var capabilities struct {
		FilesystemMode  string   `json:"filesystem_mode"`
		RootIDs         []string `json:"root_ids"`
		WritableRootIDs []string `json:"writable_root_ids"`
		WritePolicy     struct {
			Mode                                  string   `json:"mode"`
			OutputArguments                       []string `json:"output_arguments"`
			RootConfinement                       bool     `json:"root_confinement"`
			RequiresExplicitUserPathAuthorization bool     `json:"requires_explicit_user_path_authorization"`
			UsesServerProcessAccount              bool     `json:"uses_server_process_account"`
		} `json:"write_policy"`
	}
	if err := json.Unmarshal([]byte(resource.Contents[0].Text), &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.FilesystemMode != string(FilesystemModeUnrestricted) || len(capabilities.RootIDs) != 0 || len(capabilities.WritableRootIDs) != 0 {
		t.Fatalf("direct capabilities = %+v", capabilities)
	}
	if capabilities.WritePolicy.Mode != "explicit_user_authorized_path" ||
		!slices.Equal(capabilities.WritePolicy.OutputArguments, []string{"output_path"}) ||
		capabilities.WritePolicy.RootConfinement || !capabilities.WritePolicy.RequiresExplicitUserPathAuthorization ||
		!capabilities.WritePolicy.UsesServerProcessAccount {
		t.Fatalf("unrestricted write policy = %+v", capabilities.WritePolicy)
	}

	skillResource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "meido://skills/editing/com3d2.menu"})
	if err != nil || len(skillResource.Contents) != 1 {
		t.Fatalf("direct skill resource = %+v err=%v", skillResource, err)
	}
	skillText := skillResource.Contents[0].Text
	if !strings.Contains(skillText, "follow the write policy declared by the current") ||
		!strings.Contains(skillText, "Use `output_path` only for a path explicitly authorized by the user") ||
		!strings.Contains(skillText, "no configured-root confinement") ||
		!strings.Contains(skillText, "uses the server process account") ||
		strings.Contains(skillText, "Use `output_root_id`") {
		t.Fatalf("direct skill resource content = %q", skillText)
	}

	prompt, err := clientSession.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "meido.edit_format",
		Arguments: map[string]string{
			"format_id": "com3d2.menu", "objective": "Change the displayed name.",
			"input_path": menuPath, "output_path": editingPath,
		},
	})
	if err != nil || len(prompt.Messages) != 4 {
		t.Fatalf("direct editing prompt = %+v err=%v", prompt, err)
	}
	promptSkill, ok := prompt.Messages[0].Content.(*mcp.TextContent)
	if !ok || promptSkill.Text != skillText {
		t.Fatalf("unrestricted prompt and resource use different skills: prompt=%#v resource=%q", prompt.Messages[0].Content, skillText)
	}
	task, ok := prompt.Messages[3].Content.(*mcp.TextContent)
	if !ok || !strings.Contains(task.Text, "Input path: "+menuPath) || !strings.Contains(task.Text, "Output path: "+editingPath) {
		t.Fatalf("direct editing task = %#v", prompt.Messages[3].Content)
	}
}

func TestMCPFilesystemModeConfiguration(t *testing.T) {
	roots := application.NewRootSet()
	defer roots.Close()
	if err := roots.Add("mods", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if server, err := New(Config{
		Engine:         application.NewEngine(application.EngineOptions{}),
		FilesystemMode: FilesystemModeRestricted, Logger: logger,
	}); err != nil {
		t.Fatalf("explicit deny-all restricted mode: %v", err)
	} else if server.filesystemMode != FilesystemModeRestricted || len(server.roots.IDs()) != 0 {
		t.Fatalf("explicit deny-all mode = %q roots=%v", server.filesystemMode, server.roots.IDs())
	}
	if server, err := New(Config{Engine: application.NewEngine(application.EngineOptions{}), Roots: roots, Logger: logger}); err != nil {
		t.Fatalf("infer restricted mode: %v", err)
	} else if server.filesystemMode != FilesystemModeRestricted {
		t.Fatalf("mode inferred from roots = %q", server.filesystemMode)
	}
	if _, err := New(Config{
		Engine: application.NewEngine(application.EngineOptions{}), Roots: roots,
		FilesystemMode: FilesystemModeUnrestricted, Logger: logger,
	}); err == nil {
		t.Fatal("explicit unrestricted mode accepted configured roots")
	}
	if _, err := New(Config{
		Engine:         application.NewEngine(application.EngineOptions{}),
		FilesystemMode: FilesystemMode("invalid"), Logger: logger,
	}); err == nil {
		t.Fatal("invalid filesystem mode was accepted")
	}
}

func TestMCPConvertFileInstallsRawUnityBundle(t *testing.T) {
	inputDirectory := t.TempDir()
	outputDirectory := t.TempDir()
	primary := filepath.Join(inputDirectory, "hair.mmesh.bytes")
	raw := []byte{4, 0, 0, 0, 'h', 'a', 'i', 'r', 1, 2, 3}
	for path, data := range map[string][]byte{
		primary:                    raw,
		primary + ".meta.json":     []byte(`{"pathId":42,"loadName":"assets/hair"}`),
		primary + ".typetree.json": []byte(`{"format":"kces-unity-typetree","classId":43,"pathId":42,"value":{"typeName":"Mesh"}}`),
	} {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	roots := application.NewRootSet()
	defer roots.Close()
	if err := roots.Add("mods", inputDirectory); err != nil {
		t.Fatal(err)
	}
	if err := roots.AddWritable("work", outputDirectory); err != nil {
		t.Fatal(err)
	}
	server, err := New(Config{
		Engine: application.NewEngine(application.EngineOptions{}), Roots: roots,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, editingArtifact, err := server.convertFile(ctx, nil, convertInput{
		RootID: "mods", RelativePath: "hair.mmesh.bytes", FormatID: "kces.bytes", Target: "editing_json",
		OutputRootID: "work", OutputRelativePath: "editing/hair.mmesh.bytes.json",
	})
	if err != nil {
		t.Fatalf("raw to editing JSON: %v", err)
	}
	editingPath := filepath.Join(outputDirectory, "editing", "hair.mmesh.bytes.json")
	editingJSON, err := os.ReadFile(editingPath)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(editingJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	if editingArtifact.RelativePath != "editing/hair.mmesh.bytes.json" || envelope["pathId"] != float64(42) || envelope["typeTree"] == nil {
		t.Fatalf("editing output = %+v envelope=%#v", editingArtifact, envelope)
	}

	_, nativeArtifact, err := server.convertFile(ctx, nil, convertInput{
		RootID: "work", RelativePath: "editing/hair.mmesh.bytes.json", FormatID: "kces.bytes", Target: "native",
		OutputRootID: "work", OutputRelativePath: "native/hair.mmesh.bytes",
	})
	if err != nil {
		t.Fatalf("editing JSON to raw: %v", err)
	}
	if len(nativeArtifact.Attachments) != 2 {
		t.Fatalf("native artifact attachments = %+v", nativeArtifact.Attachments)
	}
	installed, err := os.ReadFile(filepath.Join(outputDirectory, "native", "hair.mmesh.bytes"))
	if err != nil || !bytes.Equal(installed, raw) {
		t.Fatalf("installed raw = %x, err=%v", installed, err)
	}
	for _, suffix := range application.ArtifactAttachmentSuffixes() {
		data, err := os.ReadFile(filepath.Join(outputDirectory, "native", "hair.mmesh.bytes") + suffix)
		if err != nil || !json.Valid(data) {
			t.Fatalf("installed %s sidecar valid=%v err=%v", suffix, json.Valid(data), err)
		}
	}
}

func TestMCPUnrestrictedConvertFileInstallsRawUnityBundle(t *testing.T) {
	directory := t.TempDir()
	primary := filepath.Join(directory, "hair.mmesh.bytes")
	raw := []byte{4, 0, 0, 0, 'h', 'a', 'i', 'r', 1, 2, 3}
	for path, data := range map[string][]byte{
		primary:                    raw,
		primary + ".meta.json":     []byte(`{"pathId":42,"loadName":"assets/hair"}`),
		primary + ".typetree.json": []byte(`{"format":"kces-unity-typetree","classId":43,"pathId":42,"value":{"typeName":"Mesh"}}`),
	} {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	server, err := New(Config{
		Engine: application.NewEngine(application.EngineOptions{}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	editingPath := filepath.Join(directory, "editing", "hair.mmesh.bytes.json")
	_, editingArtifact, err := server.convertDirectFile(ctx, nil, directConvertInput{
		Path: primary, FormatID: "kces.bytes", Target: "editing_json", OutputPath: editingPath,
	})
	if err != nil {
		t.Fatalf("raw to direct editing JSON: %v", err)
	}
	if editingArtifact.Path != editingPath || editingArtifact.RootID != "" {
		t.Fatalf("direct editing artifact = %+v", editingArtifact)
	}

	nativePath := filepath.Join(directory, "native", "hair.mmesh.bytes")
	_, nativeArtifact, err := server.convertDirectFile(ctx, nil, directConvertInput{
		Path: editingPath, FormatID: "kces.bytes", Target: "native", OutputPath: nativePath,
	})
	if err != nil {
		t.Fatalf("direct editing JSON to raw: %v", err)
	}
	if nativeArtifact.Path != nativePath || len(nativeArtifact.Attachments) != 2 {
		t.Fatalf("direct native artifact = %+v", nativeArtifact)
	}
	installed, err := os.ReadFile(nativePath)
	if err != nil || !bytes.Equal(installed, raw) {
		t.Fatalf("direct installed raw = %x err=%v", installed, err)
	}
	for _, attachment := range nativeArtifact.Attachments {
		if attachment.Path != nativePath+attachment.Suffix || attachment.RootID != "" || attachment.RelativePath != "" {
			t.Fatalf("direct attachment location = %+v", attachment)
		}
		data, readErr := os.ReadFile(attachment.Path)
		if readErr != nil || !json.Valid(data) {
			t.Fatalf("direct attachment %s valid=%v err=%v", attachment.Path, json.Valid(data), readErr)
		}
	}
}

func mcpSyntheticMenu(t *testing.T) []byte {
	t.Helper()
	menu := &serializationCOM3D2.Menu{
		Signature: serializationCOM3D2.MenuSignature, Version: 1000,
		SrcFileName: "sample.menu", ItemName: "MCP", Category: "head", InfoText: "test",
		Commands: []serializationCOM3D2.Command{{Command: "name", Args: []string{"mcp"}}},
	}
	var output bytes.Buffer
	if err := menu.Dump(&output); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func mcpContentTable(t *testing.T, count int) []byte {
	t.Helper()
	table := &ct.ContentTable{Version: 1000, Raw: make([]byte, ct.HeaderSize), Files: map[string]ct.VirtualFile{}}
	for i := 0; i < count; i++ {
		table.AddFile(fmt.Sprintf("entry-%02d.bin", i), []byte{byte(i)})
	}
	var output bytes.Buffer
	if err := ct.WriteContentTable(&output, table); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func mcpSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x", digest[:])
}
