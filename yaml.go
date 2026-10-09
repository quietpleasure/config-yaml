package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

type Yaml struct {
	data map[string]any
}

func New() *Yaml {
	return &Yaml{
		data: make(map[string]any),
	}
}

func (y *Yaml) LoadFiles(files ...string) error {
	for _, file := range files {
		dataFile := make(map[string]any)
		yamlFile, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(yamlFile, &dataFile); err != nil {
			return err
		}
		if err := mergo.Merge(&y.data, dataFile, mergo.WithOverride); err != nil {
			return err
		}
	}
	return nil
}

func (y *Yaml) LoadDir(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", pattern)
	}
	return y.LoadFiles(files...)
}

func (y *Yaml) Decode(out any) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			// Metadata:         m,
			TagName:          "yaml",
			WeaklyTypedInput: true,
			Result:           &out,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				definedAsTimeButRequiredStringHookFunc(),
			),
			// ErrorUnused: true,
			// ErrorUnset:  true,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(y.data)
}

// Param — один параметр загруженных файлов: полный ключ через точку
// (например "postgres.host") и значение в текстовом виде.
type Param struct {
	Key   string
	Value string
}

// Params возвращает все загруженные параметры плоским списком,
// отсортированным по ключу. Вложенные секции раскрываются в ключи через точку,
// списки выводятся одной строкой.
func (y *Yaml) Params() []Param {
	var params []Param
	flatten("", y.data, &params)
	sort.Slice(params, func(i, j int) bool { return params[i].Key < params[j].Key })
	return params
}

func flatten(prefix string, data map[string]any, params *[]Param) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok && len(nested) > 0 {
			flatten(key, nested, params)
			continue
		}
		*params = append(*params, Param{Key: key, Value: formatValue(v)})
	}
}

func formatValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case []any:
		items := make([]string, len(val))
		for i, item := range val {
			items[i] = formatValue(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	default:
		return fmt.Sprint(val)
	}
}

func definedAsTimeButRequiredStringHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any) (any, error) {
		if f != reflect.TypeOf(time.Time{}) {
			return data, nil
		}
		if t.Kind() != reflect.String {
			return data, nil
		}

		v, ok := data.(time.Time)
		if !ok {
			return data, fmt.Errorf("cannot convert type to structure Time")
		}
		return v.String(), nil
	}
}
