package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type LangflowCredentials struct {
	EditorAPIKey   string `json:"editor_api_key"`
	RuntimeAPIKey  string `json:"runtime_api_key"`
	RuntimeOwnerID string `json:"runtime_owner_id"`
}

func ReadLangflowCredentials(path string) (LangflowCredentials, error) {
	var c LangflowCredentials
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return c, fmt.Errorf("Langflow服务凭据文件不可用")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("Langflow服务凭据文件不可用")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || c.EditorAPIKey == "" || c.RuntimeAPIKey == "" || c.EditorAPIKey == c.RuntimeAPIKey || c.RuntimeOwnerID == "" || strings.ContainsAny(c.EditorAPIKey+c.RuntimeAPIKey, "\r\n") {
		return LangflowCredentials{}, fmt.Errorf("Langflow服务身份配置无效或未隔离")
	}
	return c, nil
}
