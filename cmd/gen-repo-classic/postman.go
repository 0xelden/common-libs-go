package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
)

const (
	postmanTemplateFolder = "OFFICE"
)

var (
	postmanNow           = time.Now
	postmanNewUUID       = func() string { return uuid.NewString() }
	postmanNewExporterID = generatePostmanExporterID
	postmanUUIDPattern   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}`)
)

type postmanCollection struct {
	Info     map[string]any   `json:"info,omitempty"`
	Item     []postmanItem    `json:"item,omitempty"`
	Event    []map[string]any `json:"event,omitempty"`
	Variable []map[string]any `json:"variable,omitempty"`
	Auth     map[string]any   `json:"auth,omitempty"`
}

type postmanItem struct {
	Name                    string           `json:"name,omitempty"`
	Item                    []postmanItem    `json:"item,omitempty"`
	Event                   []map[string]any `json:"event,omitempty"`
	Request                 *postmanRequest  `json:"request,omitempty"`
	Response                []any            `json:"response,omitempty"`
	Description             any              `json:"description,omitempty"`
	ProtocolProfileBehavior map[string]any   `json:"protocolProfileBehavior,omitempty"`
}

type postmanRequest struct {
	Auth        map[string]any   `json:"auth,omitempty"`
	Method      string           `json:"method,omitempty"`
	Header      []map[string]any `json:"header,omitempty"`
	Body        *postmanBody     `json:"body,omitempty"`
	URL         postmanURL       `json:"url"`
	Description any              `json:"description,omitempty"`
}

type postmanBody struct {
	Mode     string           `json:"mode,omitempty"`
	Formdata []map[string]any `json:"formdata,omitempty"`
	Raw      string           `json:"raw,omitempty"`
	Options  map[string]any   `json:"options,omitempty"`
}

type postmanURL struct {
	Raw      string           `json:"raw,omitempty"`
	Host     []string         `json:"host,omitempty"`
	Path     []string         `json:"path,omitempty"`
	Query    []map[string]any `json:"query,omitempty"`
	Variable []map[string]any `json:"variable,omitempty"`
}

func patchPostmanCollection(outDir, modulePath string, table *Table, urlPath string) (string, string, bool, error) {
	spec, err := inferHTTPRouteSpec(buildHTTPFile(table, modulePath), "")
	if err != nil {
		return "", "", false, err
	}
	if len(spec.Routes) == 0 {
		return "", "", false, nil
	}

	outputFileName := postmanCollectionOutputFileName(spec.ModuleName)
	outputPath := postmanCollectionPath(outDir, outputFileName)

	existingContent, err := os.ReadFile(outputPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", false, errors.Errorf("read %s: %v", outputFileName, err)
	}

	updated, changed, err := buildPostmanCollection(string(existingContent), table, spec, urlPath)
	if err != nil {
		return "", "", false, errors.Errorf("build %s: %v", outputFileName, err)
	}
	return outputFileName, updated, changed, nil
}

func postmanCollectionPath(outDir, outputFileName string) string {
	return filepath.Join(outDir, "postman", outputFileName)
}

func buildPostmanCollection(existingContent string, table *Table, spec httpRouteSpec, urlPath string) (string, bool, error) {
	templateCollection := defaultPostmanTemplateCollection()
	templateFolder, err := clonePostmanItem(defaultPostmanTemplateFolder())
	if err != nil {
		return "", false, err
	}

	targetName := postmanFolderName(spec.ModuleName)
	folder := templateFolder
	hasExistingFolder := false
	var existingCollection postmanCollection

	if strings.TrimSpace(existingContent) != "" {
		if err := json.Unmarshal([]byte(existingContent), &existingCollection); err != nil {
			return "", false, errors.Errorf("parse existing collection json: %v", err)
		}

		existingFolder, ok, err := loadExistingPostmanFolder(existingCollection, targetName)
		if err != nil {
			return "", false, err
		}
		if ok {
			folder = existingFolder
			hasExistingFolder = true
		}
	}

	templateRequests, err := postmanItemsByName(templateFolder.Item)
	if err != nil {
		return "", false, err
	}
	existingRequests, err := postmanItemsByName(folder.Item)
	if err != nil {
		return "", false, err
	}

	desired := make(map[string]struct{}, len(spec.Routes))
	merged := make([]postmanItem, 0, len(spec.Routes)+len(folder.Item))
	for _, route := range spec.Routes {
		requestName := postmanRouteName(route.Path)
		desired[strings.ToLower(requestName)] = struct{}{}

		base, ok := existingRequests[strings.ToLower(requestName)]
		if !ok {
			base, ok = templateRequests[strings.ToLower(requestName)]
		}
		if !ok {
			base = postmanItem{Name: requestName}
		}

		requestItem, err := configurePostmanRequest(base, table, spec.ModuleName, route, urlPath)
		if err != nil {
			return "", false, err
		}
		merged = append(merged, requestItem)
	}

	if hasExistingFolder {
		for _, item := range folder.Item {
			if _, ok := desired[strings.ToLower(strings.TrimSpace(item.Name))]; ok {
				continue
			}
			merged = append(merged, item)
		}
	}

	folder.Name = targetName
	folder.Item = merged

	info, err := buildPostmanCollectionInfo(templateCollection.Info, existingCollection.Info, postmanCollectionBaseName(spec.ModuleName))
	if err != nil {
		return "", false, err
	}

	collection := postmanCollection{
		Info:     info,
		Item:     []postmanItem{folder},
		Event:    templateCollection.Event,
		Variable: templateCollection.Variable,
		Auth:     templateCollection.Auth,
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return "", false, errors.Errorf("marshal collection json: %v", err)
	}

	updated := string(data)
	if existingContent == updated {
		return "", false, nil
	}
	return updated, true, nil
}

func loadExistingPostmanFolder(collection postmanCollection, targetName string) (postmanItem, bool, error) {
	if idx := findPostmanItemIndex(collection.Item, targetName); idx >= 0 {
		item, err := clonePostmanItem(collection.Item[idx])
		if err != nil {
			return postmanItem{}, false, err
		}
		return item, true, nil
	}

	if len(collection.Item) == 1 {
		item, err := clonePostmanItem(collection.Item[0])
		if err != nil {
			return postmanItem{}, false, err
		}
		return item, true, nil
	}

	return postmanItem{}, false, nil
}

func defaultPostmanTemplateCollection() postmanCollection {
	return postmanCollection{
		Info: map[string]any{
			"_collection_link": "https://go.postman.co/collection/37991928-c9c705ed-2e5b-46c3-811b-6282a27a124b?source=collection_link",
			"_exporter_id":     "26118846",
			"_postman_id":      "c9c705ed-2e5b-46c3-811b-6282a27a124b",
			"name":             "{name}",
			"schema":           "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Event: []map[string]any{
			{
				"listen": "prerequest",
				"script": map[string]any{
					"type":     "text/javascript",
					"packages": map[string]any{},
					"requests": map[string]any{},
					"exec": []string{
						"pm.request.headers.add({",
						"  key: 'Referer',",
						"  value: pm.environment.get('x_portal')",
						"});",
						"",
						"pm.request.headers.add({",
						"  key: 'X-Portal',",
						"  value: pm.environment.get('x_portal')",
						"});",
					},
				},
			},
			{
				"listen": "test",
				"script": map[string]any{
					"type":     "text/javascript",
					"packages": map[string]any{},
					"requests": map[string]any{},
					"exec":     []string{""},
				},
			},
		},
	}
}

func defaultPostmanTemplateFolder() postmanItem {
	return postmanItem{
		Name: postmanTemplateFolder,
		Item: []postmanItem{
			defaultPostmanRequestItem(
				"index",
				"GET",
				"{{gateway}}/office/index?page=1&size=10",
				[]string{"office", "index"},
				[]map[string]any{
					{"key": "page", "value": "1"},
					{"key": "size", "value": "10"},
					{"key": "filter", "value": "code = 'AAA'", "disabled": true},
					{"key": "sort", "value": "name:asc", "disabled": true},
				},
				nil,
				"//company_type_id  \na96563a8-82c2-11ee-b962-0242ac120002 CUSTOMER  \ndb290ed0-baf2-45b6-8912-5b1635ee048a DHJ  \nd6763fde-7bc9-4366-b1ae-1d99cb487576 VENDOR",
			),
			defaultPostmanRequestItem(
				"view",
				"GET",
				"{{gateway}}/office/view/6a01dbe8-59e3-4f1b-9e16-03c4c9977ff3",
				[]string{"office", "view", "6a01dbe8-59e3-4f1b-9e16-03c4c9977ff3"},
				[]map[string]any{
					{"key": "sort", "value": "name:asc", "disabled": true},
					{"key": "sort", "value": "company_id:desc", "disabled": true},
					{"key": "filter", "value": "items_id = '1bc6bc5c-a0f7-47e9-99d7-97ba1f6c1016'", "disabled": true},
				},
				nil,
				nil,
			),
			defaultPostmanRequestItem(
				"add",
				"POST",
				"{{gateway}}/office/add",
				[]string{"office", "add"},
				nil,
				&postmanBody{
					Mode: "formdata",
					Formdata: []map[string]any{
						{"key": "code", "value": "AAA-B1", "type": "text"},
						{"key": "name", "value": "PT Testing AAA Branch 1", "type": "text"},
						{"key": "address", "value": "Jl Lorem ipsum no 1 JAKARTA", "type": "text"},
						{"key": "phone", "value": "0217123123123", "type": "text"},
						{"key": "office_type", "value": "branch", "type": "text", "description": "branch, headquarters "},
						{"key": "parent_id", "value": "69e4e3b1-2fed-4bd6-bcad-d7725b95500d", "type": "text", "description": "kalo branch, wajib diisi office_id dari kantor pusat tsb"},
					},
				},
				"company type\n\ncustomer : a96563a8-82c2-11ee-b962-0242ac120002  \nvendor : d6763fde-7bc9-4366-b1ae-1d99cb487576",
			),
			defaultPostmanRequestItem(
				"edit",
				"PUT",
				"{{gateway}}/office/edit",
				[]string{"office", "edit"},
				nil,
				&postmanBody{
					Mode: "formdata",
					Formdata: []map[string]any{
						{"key": "id", "value": "6a01dbe8-59e3-4f1b-9e16-03c4c9977ff3", "type": "text", "uuid": "df32023d-eacd-4f42-9773-29f7537892b7"},
						{"key": "code", "value": "AAA-B1", "type": "text"},
						{"key": "name", "value": "PT Testing AAA - Branch 1", "type": "text"},
						{"key": "address", "value": "Jl Lorem ipsum no 1 JAKARTA SELATAN", "type": "text"},
						{"key": "phone", "value": "0217123123123", "type": "text"},
						{"key": "office_type", "value": "branch", "type": "text", "description": "branch, headquarters"},
						{"key": "parent_id", "value": "69e4e3b1-2fed-4bd6-bcad-d7725b95500d", "type": "text", "description": "kalo branch, wajib diisi office_id dari kantor pusat tsb"},
					},
				},
				"company type\n\ncustomer : a96563a8-82c2-11ee-b962-0242ac120002  \nvendor : d6763fde-7bc9-4366-b1ae-1d99cb487576",
			),
			defaultPostmanRequestItem(
				"update-status",
				"PUT",
				"{{gateway}}/office/update-status",
				[]string{"office", "update-status"},
				nil,
				&postmanBody{
					Mode: "formdata",
					Formdata: []map[string]any{
						{"key": "id", "value": "6a01dbe8-59e3-4f1b-9e16-03c4c9977ff3", "type": "text"},
						{"key": "status", "value": "0", "type": "text"},
					},
				},
				nil,
			),
			defaultPostmanRequestItem(
				"delete",
				"DELETE",
				"{{gateway}}/office/delete/25649eb6-fb17-4b10-9536-cf9946062b55",
				[]string{"office", "delete", "25649eb6-fb17-4b10-9536-cf9946062b55"},
				nil,
				nil,
				nil,
			),
		},
	}
}

func defaultPostmanRequestItem(name, method, raw string, path []string, query []map[string]any, body *postmanBody, description any) postmanItem {
	return postmanItem{
		Name: name,
		Request: &postmanRequest{
			Auth: map[string]any{
				"type": "bearer",
				"bearer": []map[string]any{
					{"key": "token", "value": "{{x_access}}", "type": "string"},
				},
			},
			Method: method,
			Header: []map[string]any{
				{"key": "Referer", "value": "{{x_portal}}", "type": "text"},
			},
			Body:        body,
			Description: description,
			URL: postmanURL{
				Raw:   raw,
				Host:  []string{"{{gateway}}"},
				Path:  path,
				Query: query,
			},
		},
		Response: []any{},
	}
}

func buildPostmanCollectionInfo(templateInfo, existingInfo map[string]any, collectionName string) (map[string]any, error) {
	info := clonePostmanInfo(templateInfo)
	if info == nil {
		info = map[string]any{}
	}

	templatePostmanID := postmanInfoString(templateInfo, "_postman_id")
	postmanID := postmanInfoString(existingInfo, "_postman_id")
	if !isValidPostmanID(postmanID) || postmanID == templatePostmanID {
		postmanID = postmanNewUUID()
	}

	templateExporterID := postmanInfoString(templateInfo, "_exporter_id")
	exporterID := postmanInfoString(existingInfo, "_exporter_id")
	if !isValidPostmanExporterID(exporterID) || exporterID == templateExporterID {
		var err error
		exporterID, err = postmanNewExporterID()
		if err != nil {
			return nil, err
		}
	}

	info["name"] = postmanCollectionName(collectionName)
	info["_postman_id"] = postmanID
	info["_exporter_id"] = exporterID

	if link := postmanCollectionLink(templateInfo, existingInfo, postmanID); link != "" {
		info["_collection_link"] = link
	}

	return info, nil
}

func clonePostmanInfo(info map[string]any) map[string]any {
	if info == nil {
		return nil
	}

	cloned := make(map[string]any, len(info))
	for key, value := range info {
		cloned[key] = value
	}
	return cloned
}

func postmanInfoString(info map[string]any, key string) string {
	if info == nil {
		return ""
	}
	value, _ := info[key].(string)
	return strings.TrimSpace(value)
}

func postmanCollectionName(base string) string {
	date := postmanNow().Format("2006-01-02")
	base = strings.TrimSpace(base)
	if base == "" {
		return date
	}
	return base + " " + date
}

func postmanCollectionBaseName(moduleName string) string {
	return strings.TrimSuffix(postmanCollectionOutputFileName(moduleName), ".collections.json")
}

func postmanCollectionLink(templateInfo, existingInfo map[string]any, postmanID string) string {
	link := postmanInfoString(existingInfo, "_collection_link")
	if link == "" {
		link = postmanInfoString(templateInfo, "_collection_link")
	}
	if link == "" {
		return ""
	}

	currentPostmanID := postmanInfoString(existingInfo, "_postman_id")
	if !isValidPostmanID(currentPostmanID) {
		currentPostmanID = postmanInfoString(templateInfo, "_postman_id")
	}

	if currentPostmanID != "" && strings.Contains(link, currentPostmanID) {
		return strings.Replace(link, currentPostmanID, postmanID, 1)
	}
	return postmanUUIDPattern.ReplaceAllString(link, postmanID)
}

func generatePostmanExporterID() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(90000000))
	if err != nil {
		return "", errors.Errorf("generate postman exporter id: %v", err)
	}
	return fmt.Sprintf("%08d", value.Int64()+10000000), nil
}

func isValidPostmanID(value string) bool {
	if value == "" {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil
}

func isValidPostmanExporterID(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func postmanCollectionOutputFileName(moduleName string) string {
	return moduleName + ".collections.json"
}

func configurePostmanRequest(base postmanItem, table *Table, moduleName string, route httpRoute, urlPath string) (postmanItem, error) {
	item, err := clonePostmanItem(base)
	if err != nil {
		return postmanItem{}, err
	}

	requestName := postmanRouteName(route.Path)
	item.Name = requestName
	if item.Request == nil {
		item.Request = &postmanRequest{}
	}
	item.Request.Method = route.Method
	item.Request.Description = nil
	item.Request.URL = buildPostmanURL(moduleName, route, table, item.Request.URL, urlPath)

	switch requestName {
	case "add":
		item.Request.Body = &postmanBody{
			Mode:     "formdata",
			Formdata: buildPostmanFormData(table, false),
		}
	case "edit":
		item.Request.Body = &postmanBody{
			Mode:     "formdata",
			Formdata: buildPostmanFormData(table, true),
		}
	case "update-status":
		item.Request.Body = &postmanBody{
			Mode:     "formdata",
			Formdata: buildPostmanStatusFormData(table),
		}
	default:
		item.Request.Body = nil
	}

	return item, nil
}

func buildPostmanURL(moduleName string, route httpRoute, table *Table, existing postmanURL, urlPath string) postmanURL {
	segments := postmanPathSegments(moduleName, route.Path, table, existing.Path, urlPath)
	query := buildPostmanQuery(route, table)

	url := existing
	url.Host = []string{"{{gateway}}"}
	url.Path = segments
	url.Query = query
	url.Raw = buildPostmanRawURL(url.Host, segments, query)
	return url
}

func buildPostmanRawURL(host, path []string, query []map[string]any) string {
	raw := "{{gateway}}"
	if len(host) > 0 && host[0] != "" {
		raw = host[0]
	}
	if len(path) > 0 {
		raw += "/" + strings.Join(path, "/")
	}

	activeQuery := make([]string, 0, len(query))
	for _, part := range query {
		key, _ := part["key"].(string)
		if key == "" || postmanBool(part["disabled"]) {
			continue
		}
		value, _ := part["value"].(string)
		activeQuery = append(activeQuery, fmt.Sprintf("%s=%s", key, value))
	}
	if len(activeQuery) > 0 {
		raw += "?" + strings.Join(activeQuery, "&")
	}
	return raw
}

func postmanPathSegments(moduleName, routePath string, table *Table, existing []string, urlPath string) []string {
	parts := strings.Split(strings.TrimPrefix(routePath, "/"), "/")
	prefix := postmanURLPathPrefix(urlPath)
	result := make([]string, 0, len(prefix)+len(parts)+1)
	result = append(result, prefix...)
	result = append(result, moduleName)

	for idx, part := range parts {
		if strings.HasPrefix(part, ":") {
			if len(existing) > idx+1 && existing[idx+1] != "" && !strings.HasPrefix(existing[idx+1], ":") {
				result = append(result, existing[idx+1])
				continue
			}
			result = append(result, samplePathValue(table))
			continue
		}
		result = append(result, part)
	}
	return result
}

func postmanURLPathPrefix(urlPath string) []string {
	urlPath = strings.Trim(strings.TrimSpace(urlPath), "/")
	if urlPath == "" {
		return nil
	}

	parts := strings.Split(urlPath, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func buildPostmanQuery(route httpRoute, table *Table) []map[string]any {
	switch postmanRouteName(route.Path) {
	case "index":
		return defaultPostmanIndexQuery(table)
	default:
		return nil
	}
}

func defaultPostmanIndexQuery(table *Table) []map[string]any {
	result := []map[string]any{
		{"key": "page", "value": "1"},
		{"key": "size", "value": "10"},
	}

	if filter := defaultPostmanFilterValue(table); filter != "" {
		result = append(result, map[string]any{"key": "filter", "value": filter, "disabled": true})
	}
	if sort := defaultPostmanSortValue(table); sort != "" {
		result = append(result, map[string]any{"key": "sort", "value": sort, "disabled": true})
	}

	return result
}

func defaultPostmanFilterValue(table *Table) string {
	if table == nil {
		return ""
	}

	for _, col := range table.Columns {
		if isAuditColumn(col.Name) || isIDColumn(col) {
			continue
		}
		return postmanFilterExpression(col)
	}
	return ""
}

func postmanFilterExpression(col Column) string {
	base := baseSQLType(col.RawType)
	switch {
	case strings.Contains(base, "uuid"):
		return fmt.Sprintf("%s = '%s'", col.Name, sampleUUIDValue())
	case base == "bigint" || strings.Contains(base, "bigserial"):
		return fmt.Sprintf("%s = 1", col.Name)
	case strings.Contains(base, "smallint"), strings.Contains(base, "int"), strings.Contains(base, "serial"):
		return fmt.Sprintf("%s = 1", col.Name)
	case strings.Contains(base, "float"), strings.Contains(base, "numeric"), strings.Contains(base, "decimal"), strings.Contains(base, "double"), strings.Contains(base, "real"):
		return fmt.Sprintf("%s = 1", col.Name)
	case strings.Contains(base, "bool"):
		return fmt.Sprintf("%s = true", col.Name)
	case strings.Contains(base, "timestamp"), strings.Contains(base, "timestamptz"), strings.Contains(base, "datetime"), base == "date", base == "time", base == "timetz":
		return fmt.Sprintf("%s = '2026-01-01'", col.Name)
	default:
		return fmt.Sprintf("%s = 'sample'", col.Name)
	}
}

func defaultPostmanSortValue(table *Table) string {
	if table == nil {
		return ""
	}

	for _, preferred := range []string{"created_at", "updated_at", "id"} {
		for _, col := range table.Columns {
			if strings.EqualFold(col.Name, preferred) {
				return col.Name + ":desc"
			}
		}
	}

	for _, col := range table.Columns {
		if isAuditColumn(col.Name) {
			continue
		}
		return col.Name + ":asc"
	}
	return ""
}

func isAuditColumn(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(lower, "created_"):
		return true
	case strings.HasPrefix(lower, "updated_"):
		return true
	case strings.HasPrefix(lower, "deleted_"):
		return true
	default:
		return false
	}
}

func buildPostmanFormData(table *Table, includeID bool) []map[string]any {
	if table == nil {
		return nil
	}

	fields := make([]Column, 0, len(table.Columns))
	if includeID {
		for _, col := range table.Columns {
			if isIDColumn(col) {
				fields = append(fields, col)
				break
			}
		}
	}

	for _, col := range table.Columns {
		if isIDColumn(col) {
			continue
		}
		if col.IsGenerated {
			continue
		}
		if isAuditColumn(col.Name) {
			continue
		}
		fields = append(fields, col)
	}

	formdata := make([]map[string]any, 0, len(fields))
	for _, col := range fields {
		formdata = append(formdata, map[string]any{
			"key":   col.Name,
			"value": postmanSampleValue(col),
			"type":  "text",
		})
	}
	return formdata
}

func buildPostmanStatusFormData(table *Table) []map[string]any {
	result := make([]map[string]any, 0, 2)
	for _, col := range table.Columns {
		if isIDColumn(col) {
			result = append(result, map[string]any{
				"key":   col.Name,
				"value": postmanSampleValue(col),
				"type":  "text",
			})
			break
		}
	}
	result = append(result, map[string]any{
		"key":   "status",
		"value": "1",
		"type":  "text",
	})
	return result
}

func postmanSampleValue(col Column) string {
	name := strings.ToLower(col.Name)
	switch {
	case isIDColumn(col):
		return samplePathValue(nil)
	case strings.Contains(name, "email"):
		return "sample@example.com"
	case strings.Contains(name, "phone"):
		return "08123456789"
	case strings.Contains(name, "status"):
		return "1"
	}

	base := baseSQLType(col.RawType)
	switch {
	case strings.Contains(base, "uuid"):
		return sampleUUIDValue()
	case base == "bigint" || strings.Contains(base, "bigserial"):
		return "1"
	case strings.Contains(base, "smallint"), strings.Contains(base, "int"), strings.Contains(base, "serial"):
		return "1"
	case strings.Contains(base, "float"), strings.Contains(base, "numeric"), strings.Contains(base, "decimal"), strings.Contains(base, "double"), strings.Contains(base, "real"):
		return "1"
	case strings.Contains(base, "bool"):
		return "true"
	case strings.Contains(base, "timestamp"), strings.Contains(base, "timestamptz"), strings.Contains(base, "datetime"), base == "date", base == "time", base == "timetz":
		return "2026-01-01T00:00:00Z"
	case strings.Contains(base, "json"):
		return "{}"
	default:
		return col.Name
	}
}

func samplePathValue(table *Table) string {
	if table != nil {
		for _, col := range table.Columns {
			if !isIDColumn(col) {
				continue
			}
			if strings.Contains(baseSQLType(col.RawType), "uuid") {
				return sampleUUIDValue()
			}
			return "1"
		}
	}
	return sampleUUIDValue()
}

func sampleUUIDValue() string {
	return "00000000-0000-0000-0000-000000000001"
}

func isIDColumn(col Column) bool {
	return col.IsPrimary || strings.EqualFold(col.Name, "id")
}

func postmanRouteName(routePath string) string {
	parts := strings.Split(strings.TrimPrefix(routePath, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "" || strings.HasPrefix(parts[i], ":") {
			continue
		}
		return parts[i]
	}
	return ""
}

func postmanFolderName(moduleName string) string {
	return strings.ToUpper(strings.ReplaceAll(moduleName, "_", " "))
}

func postmanItemsByName(items []postmanItem) (map[string]postmanItem, error) {
	result := make(map[string]postmanItem, len(items))
	for _, item := range items {
		cloned, err := clonePostmanItem(item)
		if err != nil {
			return nil, err
		}
		result[strings.ToLower(strings.TrimSpace(item.Name))] = cloned
	}
	return result, nil
}

func findPostmanItemIndex(items []postmanItem, name string) int {
	for idx, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(name)) {
			return idx
		}
	}
	return -1
}

func clonePostmanItem(item postmanItem) (postmanItem, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return postmanItem{}, errors.Errorf("marshal postman item: %v", err)
	}

	var cloned postmanItem
	if err = json.Unmarshal(data, &cloned); err != nil {
		return postmanItem{}, errors.Errorf("unmarshal postman item: %v", err)
	}
	return cloned, nil
}

func postmanBool(value any) bool {
	flag, ok := value.(bool)
	return ok && flag
}
