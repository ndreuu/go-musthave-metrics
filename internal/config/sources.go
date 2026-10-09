// Package config contains helpers shared by the agent and server configuration loaders.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"dario.cat/mergo"
)

// Pointer marks a value as present, including zero, false and empty strings.
func Pointer[T any](value T) *T { return &value }

// Merge applies sources in increasing priority. Source structs use pointer fields:
// nil means absent, while a non-nil pointer can explicitly override with zero.
func Merge[T any](sources ...T) (T, error) {
	var result T
	for _, source := range sources {
		if err := mergo.Merge(&result, source, mergo.WithOverride, mergo.WithoutDereference); err != nil {
			return result, err
		}
	}
	return result, nil
}

// ExplicitFlags removes flag defaults from a source, leaving only flags supplied
// by the user. Fields identify their CLI flag with the flag struct tag.
func ExplicitFlags(source any, flags *flag.FlagSet) {
	set := make(map[string]bool)
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	value := reflect.ValueOf(source).Elem()
	for i := 0; i < value.NumField(); i++ {
		if !set[value.Type().Field(i).Tag.Get("flag")] {
			value.Field(i).SetZero()
		}
	}
}

// Environment reads the env tags of a pointer-field source struct. As with the
// existing CLI behavior, unset and empty environment variables are ignored.
// Fields already present in overridden are skipped to respect higher priority.
func Environment[T any](lookup func(string) (string, bool), overridden T) (T, error) {
	var source T
	value := reflect.ValueOf(&source).Elem()
	override := reflect.ValueOf(overridden)
	for i := 0; i < value.NumField(); i++ {
		if !override.Field(i).IsNil() {
			continue
		}
		name := value.Type().Field(i).Tag.Get("env")
		raw, ok := lookup(name)
		if !ok || raw == "" {
			continue
		}
		field := value.Field(i)
		parsed := reflect.New(field.Type().Elem())
		switch parsed.Elem().Kind() {
		case reflect.String:
			parsed.Elem().SetString(raw)
		case reflect.Int:
			integer, err := strconv.Atoi(raw)
			if err != nil {
				return source, fmt.Errorf("invalid %s value %q: %w", name, raw, err)
			}
			parsed.Elem().SetInt(int64(integer))
		case reflect.Bool:
			parsed.Elem().SetBool(raw == "true")
		default:
			return source, fmt.Errorf("unsupported configuration field %s", value.Type().Field(i).Name)
		}
		field.Set(parsed)
	}
	return source, nil
}

// ReadFile loads the optional JSON file. An explicitly provided empty -c or
// -config disables CONFIG from the environment.
func ReadFile(flags *flag.FlagSet, path string, lookup func(string) (string, bool), destination any) error {
	explicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "c" || f.Name == "config" {
			explicit = true
		}
	})
	if !explicit {
		path, _ = lookup("CONFIG")
	}
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	return nil
}

// Duration accepts Go duration strings and integer seconds, including zero.
func Duration(value string) (time.Duration, error) {
	if _, err := strconv.Atoi(value); err == nil {
		value += "s"
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: %w", value, err)
	}
	return duration, nil
}
