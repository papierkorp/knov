package configmanager

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"knov/internal/logging"
)

// StorableSetting is implemented by all settings for persistence and value access.
type StorableSetting interface {
	Key() string
	GetValue() interface{}
	setFromJSON(v interface{})
	SetFromString(s string) error
}

// RenderableSetting extends StorableSetting with UI metadata for auto-rendering.
type RenderableSetting interface {
	StorableSetting
	Type() string // "boolean"|"number"|"text"|"select"|"dynamic-select"|"textarea"
	GetMeta() Meta
}

// Meta holds display and behaviour metadata returned by GetMeta.
// It is assembled on demand — meta fields live directly in each setting struct.
type Meta struct {
	Section  SettingSection
	Group    SettingGroup
	Label    string
	Desc     string
	Trigger  string
	Target   string
	Options  []SettingOption
	DynURL   string
	Min, Max *int
	Refresh  bool // when true, the POST handler responds with HX-Refresh: true
}

// ── registry ──────────────────────────────────────────────────────────────────

var allSettings []StorableSetting
var renderSettings []RenderableSetting
var settingsByKey = make(map[string]StorableSetting)

func registerSetting(s StorableSetting) {
	if _, exists := settingsByKey[s.Key()]; exists {
		panic(fmt.Sprintf("configmanager: duplicate setting key %q", s.Key()))
	}
	allSettings = append(allSettings, s)
	settingsByKey[s.Key()] = s
	if r, ok := any(s).(RenderableSetting); ok {
		renderSettings = append(renderSettings, r)
	}
}

// register adds s to the global registry and returns it with its concrete type intact.
func register[T StorableSetting](s T) T {
	registerSetting(s)
	return s
}

// AllSettings returns all settings in registration order.
func AllSettings() []StorableSetting { return allSettings }

// GetSetting returns a setting by key, or nil if not found.
func GetSetting(key string) StorableSetting { return settingsByKey[key] }

// intPtr is a helper for Min/Max fields.
func intPtr(n int) *int { return &n }

// ── BoolSetting ───────────────────────────────────────────────────────────────

type BoolSetting struct {
	key     string
	Default bool
	val     atomic.Pointer[bool]
	Section  SettingSection
	Group    SettingGroup
	Label    string
	Desc     string
	Trigger  string
	Target   string
	OnChange func(interface{})
}

func (s *BoolSetting) Get() bool {
	if p := s.val.Load(); p != nil {
		return *p
	}
	return s.Default
}
func (s *BoolSetting) Key() string           { return s.key }
func (s *BoolSetting) Type() string          { return "boolean" }
func (s *BoolSetting) GetValue() interface{} { return s.Get() }
func (s *BoolSetting) GetMeta() Meta {
	return Meta{Section: s.Section, Group: s.Group, Label: s.Label, Desc: s.Desc, Trigger: s.Trigger, Target: s.Target}
}
func (s *BoolSetting) setFromJSON(v interface{}) {
	if b, ok := v.(bool); ok {
		s.val.Store(&b)
	}
}
func (s *BoolSetting) SetFromString(v string) error {
	b, _ := strconv.ParseBool(v) // empty string → false (unchecked checkbox)
	s.val.Store(&b)
	if s.OnChange != nil {
		s.OnChange(b)
	}
	return nil
}

// ── IntSetting ────────────────────────────────────────────────────────────────

type IntSetting struct {
	key      string
	Default  int
	val      atomic.Pointer[int]
	Section  SettingSection
	Group    SettingGroup
	Label    string
	Desc     string
	Trigger  string
	Target   string
	Min, Max *int
	OnChange func(interface{})
	Validate func(int) error
}

func (s *IntSetting) Get() int {
	if p := s.val.Load(); p != nil {
		return *p
	}
	return s.Default
}
func (s *IntSetting) Key() string           { return s.key }
func (s *IntSetting) Type() string          { return "number" }
func (s *IntSetting) GetValue() interface{} { return s.Get() }
func (s *IntSetting) GetMeta() Meta {
	return Meta{Section: s.Section, Group: s.Group, Label: s.Label, Desc: s.Desc, Trigger: s.Trigger, Target: s.Target, Min: s.Min, Max: s.Max}
}
func (s *IntSetting) validate(n int) error {
	if s.Min != nil && n < *s.Min {
		return fmt.Errorf("value %d is below minimum %d", n, *s.Min)
	}
	if s.Max != nil && n > *s.Max {
		return fmt.Errorf("value %d exceeds maximum %d", n, *s.Max)
	}
	if s.Validate != nil {
		return s.Validate(n)
	}
	return nil
}
func (s *IntSetting) setFromJSON(v interface{}) {
	var n int
	switch val := v.(type) {
	case float64:
		n = int(val)
	case int:
		n = val
	default:
		return
	}
	if err := s.validate(n); err != nil {
		logging.LogWarning("setting %q: ignoring stored value %d: %v", s.key, n, err)
		return
	}
	s.val.Store(&n)
}
func (s *IntSetting) SetFromString(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid integer: %q", v)
	}
	if err := s.validate(n); err != nil {
		return err
	}
	s.val.Store(&n)
	if s.OnChange != nil {
		s.OnChange(n)
	}
	return nil
}

// ── StringSetting ─────────────────────────────────────────────────────────────

type StringSetting struct {
	key      string
	Default  string
	val      atomic.Pointer[string]
	Section  SettingSection
	Group    SettingGroup
	Label    string
	Desc     string
	Trigger  string
	Target   string
	Options  []SettingOption
	DynURL   string
	Refresh  bool
	OnChange func(interface{})
	Validate func(string) error
}

func (s *StringSetting) Get() string {
	if p := s.val.Load(); p != nil {
		return *p
	}
	return s.Default
}
func (s *StringSetting) Key() string           { return s.key }
func (s *StringSetting) GetValue() interface{} { return s.Get() }
func (s *StringSetting) GetMeta() Meta {
	return Meta{Section: s.Section, Group: s.Group, Label: s.Label, Desc: s.Desc, Trigger: s.Trigger, Target: s.Target, Options: s.Options, DynURL: s.DynURL, Refresh: s.Refresh}
}
func (s *StringSetting) validate(v string) error {
	if len(s.Options) > 0 {
		for _, o := range s.Options {
			if o.Value == v {
				return nil
			}
		}
		return fmt.Errorf("invalid value %q", v)
	}
	if s.Validate != nil {
		return s.Validate(v)
	}
	return nil
}
func (s *StringSetting) setFromJSON(v interface{}) {
	str, ok := v.(string)
	if !ok {
		return
	}
	if err := s.validate(str); err != nil {
		logging.LogWarning("setting %q: ignoring stored value %q: %v", s.key, str, err)
		return
	}
	s.val.Store(&str)
}
func (s *StringSetting) SetFromString(v string) error {
	if err := s.validate(v); err != nil {
		return err
	}
	s.val.Store(&v)
	if s.OnChange != nil {
		s.OnChange(v)
	}
	return nil
}
func (s *StringSetting) Type() string {
	if len(s.Options) > 0 {
		return "select"
	}
	if s.DynURL != "" {
		return "dynamic-select"
	}
	return "text"
}

// ── StringSliceSetting ────────────────────────────────────────────────────────

type StringSliceSetting struct {
	key     string
	Default []string
	val     atomic.Pointer[[]string]
	Section  SettingSection
	Group    SettingGroup
	Label    string
	Desc     string
	Trigger  string
	Target   string
	OnChange func(interface{})
}

func (s *StringSliceSetting) Get() []string {
	if p := s.val.Load(); p != nil {
		return *p
	}
	return s.Default
}
func (s *StringSliceSetting) Key() string           { return s.key }
func (s *StringSliceSetting) Type() string          { return "textarea" }
func (s *StringSliceSetting) GetValue() interface{} { return s.Get() }
func (s *StringSliceSetting) GetMeta() Meta {
	return Meta{Section: s.Section, Group: s.Group, Label: s.Label, Desc: s.Desc, Trigger: s.Trigger, Target: s.Target}
}
func (s *StringSliceSetting) setFromJSON(v interface{}) {
	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		s.val.Store(&result)
	case []string:
		s.val.Store(&val)
	}
}
func (s *StringSliceSetting) SetFromString(v string) error {
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	s.val.Store(&result)
	if s.OnChange != nil {
		s.OnChange(result)
	}
	return nil
}

// ── MapSetting ────────────────────────────────────────────────────────────────

// MapSetting holds an arbitrary structured value serialised as JSON.
// Set uses copy-on-write: it stores a pointer to a new value, so concurrent
// Get calls always see a complete, immutable snapshot with no locking needed.
//
// MapSetting intentionally does NOT implement RenderableSetting — it is
// persisted via allSettings but never auto-rendered on the settings page.
// Mutations go through dedicated accessor functions (e.g. SetThemeSetting),
// not the generic POST /api/settings/{key} handler.
type MapSetting[T any] struct {
	key     string
	Default T
	val     atomic.Pointer[T]
}

func (s *MapSetting[T]) Get() T {
	if p := s.val.Load(); p != nil {
		return *p
	}
	return s.Default
}
func (s *MapSetting[T]) Set(v T) {
	s.val.Store(&v)
}
func (s *MapSetting[T]) Key() string           { return s.key }
func (s *MapSetting[T]) GetValue() interface{} { return s.Get() }
func (s *MapSetting[T]) setFromJSON(v interface{}) {
	if v == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return
	}
	s.val.Store(&t)
}
func (s *MapSetting[T]) SetFromString(string) error { return nil }
