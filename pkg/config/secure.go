package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"getytstatsapi/pkg/credential"
	"gopkg.in/yaml.v3"
)

const notHere = `"[NOT_HERE]"`

type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	var singleString string
	if err := json.Unmarshal(data, &singleString); err == nil {
		*f = FlexibleStringSlice{singleString}
		return nil
	}

	var singleNumber float64
	if err := json.Unmarshal(data, &singleNumber); err == nil {
		*f = FlexibleStringSlice{fmt.Sprintf("%.0f", singleNumber)}
		return nil
	}

	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		*f = ss
		return nil
	}

	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		var s string
		if err = json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = []string{s}
		return nil
	}

	result := make([]string, 0, len(raw))
	for _, v := range raw {
		switch val := v.(type) {
		case string:
			result = append(result, val)
		case float64:
			result = append(result, fmt.Sprintf("%.0f", val))
		default:
			result = append(result, fmt.Sprintf("%v", val))
		}
	}
	*f = result
	return nil
}

func (f *FlexibleStringSlice) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*f = nil
		return nil
	}
	s := strings.ReplaceAll(string(text), "，", ",")
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	*f = result
	return nil
}

type SecureStrings []*SecureString

func (s *SecureStrings) Values() []string {
	if s == nil {
		return nil
	}
	keys := make([]string, len(*s))
	for i, k := range *s {
		keys[i] = k.String()
	}
	return unique(keys)
}

func SimpleSecureStrings(val ...string) SecureStrings {
	val = unique(val)
	vv := make(SecureStrings, len(val))
	for i, s := range val {
		vv[i] = NewSecureString(s)
	}
	return vv
}

func (s SecureStrings) MarshalJSON() ([]byte, error) {
	return []byte(notHere), nil
}

func (s *SecureStrings) UnmarshalJSON(value []byte) error {
	if string(value) == notHere {
		return nil
	}
	var v []*SecureString
	if err := json.Unmarshal(value, &v); err != nil {
		return err
	}
	*s = v
	return nil
}

type SecureString struct {
	resolved string
	raw      string
}

func NewSecureString(value string) *SecureString {
	s := &SecureString{}
	_ = s.fromRaw(value)
	return s
}

func (s *SecureString) String() string {
	if s == nil {
		return ""
	}
	return s.resolved
}

func (s *SecureString) Set(value string) *SecureString {
	s.resolved = value
	s.raw = ""
	return s
}

func (s *SecureString) IsZero() bool {
	if callerFromYAML() {
		return true
	}
	return s.resolved == ""
}

func (s *SecureString) MarshalJSON() ([]byte, error) {
	return []byte(notHere), nil
}

func (s *SecureString) UnmarshalJSON(value []byte) error {
	if string(value) == notHere {
		return nil
	}
	var v string
	if err := json.Unmarshal(value, &v); err != nil {
		return err
	}
	return s.fromRaw(v)
}

func (s *SecureString) MarshalYAML() (any, error) {
	if strings.HasPrefix(s.raw, credential.EncScheme) || strings.HasPrefix(s.raw, credential.FileScheme) {
		return s.raw, nil
	}
	if strings.HasPrefix(s.resolved, credential.EncScheme) || strings.HasPrefix(s.resolved, credential.FileScheme) {
		return s.resolved, nil
	}
	if passphrase := credential.PassphraseProvider(); passphrase != "" {
		encrypted, err := credential.Encrypt(passphrase, "", s.resolved)
		if err != nil {
			return nil, err
		}
		return encrypted, nil
	}
	return s.resolved, nil
}

func (s *SecureString) UnmarshalYAML(value *yaml.Node) error {
	return s.fromRaw(value.Value)
}

func (s *SecureString) UnmarshalText(text []byte) error {
	return s.fromRaw(string(text))
}

func (s *SecureString) fromRaw(v string) error {
	s.raw = v
	vv, err := resolveKey(v)
	if err != nil {
		return err
	}
	s.resolved = vv
	return nil
}

var (
	secResolverMu sync.RWMutex
	secResolver   *credential.Resolver
)

func setCredentialResolverBaseDir(path string) {
	secResolverMu.Lock()
	defer secResolverMu.Unlock()
	secResolver = credential.NewResolver(path)
}

func resolveKey(v string) (string, error) {
	secResolverMu.RLock()
	resolver := secResolver
	secResolverMu.RUnlock()
	if resolver == nil {
		resolver = credential.NewResolver("")
	}
	if strings.HasPrefix(v, credential.EncScheme) || strings.HasPrefix(v, credential.FileScheme) {
		return resolver.Resolve(v)
	}
	return v, nil
}

func callerFromYAML() bool {
	_, file, _, ok := runtime.Caller(2)
	if !ok {
		return false
	}
	return !strings.Contains(filepath.Dir(file), "yaml.v")
}

func unique[T comparable](input []T) []T {
	m := make(map[T]struct{}, len(input))
	var result []T
	for _, v := range input {
		if _, ok := m[v]; !ok {
			m[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
