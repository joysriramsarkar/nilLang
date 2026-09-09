package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

const ImageFormat = "nil-self-host-image"

var CompilerModules = []string{"lexer.nil", "parser.nil", "analyzer.nil", "emitter.nil"}

type Module struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type Image struct {
	Format   string         `json:"format"`
	Version  int            `json:"version"`
	Modules  []Module       `json:"modules"`
	Artifact map[string]any `json:"artifact"`
}

func BuildSeed(directory string) ([]byte, error) {
	modules := make([]Module, 0, len(CompilerModules))
	for _, name := range CompilerModules {
		source, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("read compiler module %s: %w", name, err)
		}
		modules = append(modules, Module{Name: name, Source: string(source)})
	}
	return BuildImage(modules)
}

func Rebuild(encoded []byte) ([]byte, error) {
	var image Image
	if err := json.Unmarshal(encoded, &image); err != nil {
		return nil, fmt.Errorf("decode compiler image: %w", err)
	}
	if image.Format != ImageFormat || image.Version != 1 {
		return nil, fmt.Errorf("unsupported compiler image %q version %d", image.Format, image.Version)
	}
	return BuildImage(image.Modules)
}

func BuildImage(modules []Module) ([]byte, error) {
	var compilerSource bytes.Buffer
	for _, module := range modules {
		compilerSource.WriteString(module.Source)
		compilerSource.WriteByte('\n')
	}
	compilerSource.WriteString("nilCompile(bootstrapInput);")

	parsed := parser.New(lexer.New(compilerSource.String()))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		return nil, fmt.Errorf("parse self-hosted compiler: %v", parsed.Errors())
	}

	environment := object.NewEnvironment()
	environment.Set("bootstrapInput", &object.String{Value: compilerSource.String()[:compilerSource.Len()-len("nilCompile(bootstrapInput);")-1]})
	result := evaluator.Eval(program, environment)
	if result == nil {
		return nil, fmt.Errorf("self-hosted compiler returned no artifact")
	}
	if result.Type() == object.ERROR_OBJ {
		return nil, fmt.Errorf("run self-hosted compiler: %s", result.Inspect())
	}

	artifact, ok := objectToValue(result).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("self-hosted compiler returned %s, want hash", result.Type())
	}
	if artifact["ok"] != true {
		return nil, fmt.Errorf("self-hosted compiler rejected its source: %v", artifact)
	}

	image := Image{Format: ImageFormat, Version: 1, Modules: modules, Artifact: artifact}
	encoded, err := json.MarshalIndent(image, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode compiler image: %w", err)
	}
	return append(encoded, '\n'), nil
}

func objectToValue(value object.Object) any {
	switch value := value.(type) {
	case *object.Null:
		return nil
	case *object.Boolean:
		return value.Value
	case *object.Integer:
		return value.Value
	case *object.Float:
		return value.Value
	case *object.String:
		return value.Value
	case *object.Array:
		items := make([]any, len(value.Elements))
		for index, element := range value.Elements {
			items[index] = objectToValue(element)
		}
		return items
	case *object.Hash:
		items := make(map[string]any, len(value.Pairs))
		for _, pair := range value.Pairs {
			key, ok := pair.Key.(*object.String)
			if !ok {
				return nil
			}
			items[key.Value] = objectToValue(pair.Value)
		}
		return items
	default:
		return nil
	}
}
