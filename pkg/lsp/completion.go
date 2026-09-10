package lsp

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

var defaultKeywordCompletions = []CompletionItem{
	{Label: "let", Kind: CompletionKindKeyword, Detail: "Variable declaration", InsertText: "let ${1:name} = ${2:value};"},
	{Label: "const", Kind: CompletionKindKeyword, Detail: "Constant declaration", InsertText: "const ${1:NAME} = ${2:value};"},
	{Label: "fn", Kind: CompletionKindSnippet, Detail: "Function definition", InsertText: "fn ${1:name}(${2:params}) {\n    ${3:// body}\n}"},
	{Label: "entity", Kind: CompletionKindSnippet, Detail: "Entity definition", InsertText: "entity ${1:Name} {\n    id: UUID primary;\n    ${2}\n}"},
	{Label: "component", Kind: CompletionKindSnippet, Detail: "Alap Component", InsertText: "component ${1:Name} {\n    state count = 0;\n    render {\n        ${2}\n    }\n}"},
	{Label: "state", Kind: CompletionKindKeyword, Detail: "Component state", InsertText: "state ${1:name} = ${2:value};"},
	{Label: "render", Kind: CompletionKindKeyword, Detail: "Render block", InsertText: "render {\n    ${1}\n}"},
	{Label: "build", Kind: CompletionKindKeyword, Detail: "Build block", InsertText: "build {\n    ${1}\n}"},
	{Label: "on", Kind: CompletionKindKeyword, Detail: "Event handler", InsertText: "on ${1:click} {\n    ${2}\n}"},
	{Label: "while", Kind: CompletionKindSnippet, Detail: "While loop", InsertText: "while (${1:condition}) {\n    ${2}\n}"},
	{Label: "if", Kind: CompletionKindSnippet, Detail: "If statement", InsertText: "if (${1:condition}) {\n    ${2}\n}"},
	{Label: "else", Kind: CompletionKindKeyword, Detail: "Else clause", InsertText: "else {\n    ${1}\n}"},
	{Label: "return", Kind: CompletionKindKeyword, Detail: "Return statement", InsertText: "return ${1:value};"},
	{Label: "import", Kind: CompletionKindSnippet, Detail: "Import statement", InsertText: "import { ${1:symbols} } from \"${2:path}\";"},
	{Label: "true", Kind: CompletionKindValue, Detail: "Boolean true", InsertText: "true"},
	{Label: "false", Kind: CompletionKindValue, Detail: "Boolean false", InsertText: "false"},
	{Label: "null", Kind: CompletionKindValue, Detail: "Null literal", InsertText: "null"},
}

var defaultBuiltinCompletions = []CompletionItem{
	{Label: "println", Kind: CompletionKindFunction, Detail: "println(args...) -> void", InsertText: "println(${1:expr});"},
	{Label: "print", Kind: CompletionKindFunction, Detail: "print(args...) -> void", InsertText: "print(${1:expr});"},
	{Label: "puts", Kind: CompletionKindFunction, Detail: "puts(args...) -> void", InsertText: "puts(${1:expr});"},
	{Label: "len", Kind: CompletionKindFunction, Detail: "len(iterable) -> Int", InsertText: "len(${1:iterable})"},
	{Label: "push", Kind: CompletionKindFunction, Detail: "push(arr, item) -> Array", InsertText: "push(${1:arr}, ${2:item})"},
	{Label: "first", Kind: CompletionKindFunction, Detail: "first(arr) -> Any", InsertText: "first(${1:arr})"},
	{Label: "last", Kind: CompletionKindFunction, Detail: "last(arr) -> Any", InsertText: "last(${1:arr})"},
	{Label: "rest", Kind: CompletionKindFunction, Detail: "rest(arr) -> Array", InsertText: "rest(${1:arr})"},
	{Label: "type", Kind: CompletionKindFunction, Detail: "type(val) -> String", InsertText: "type(${1:val})"},
	{Label: "assert", Kind: CompletionKindFunction, Detail: "assert(cond, msg) -> void", InsertText: "assert(${1:condition});"},
}

// ComputeCompletions aggregates keywords, builtins, and declared AST symbols.
func ComputeCompletions(content string) CompletionList {
	items := make([]CompletionItem, 0, len(defaultKeywordCompletions)+len(defaultBuiltinCompletions)+16)
	items = append(items, defaultKeywordCompletions...)
	items = append(items, defaultBuiltinCompletions...)

	seen := make(map[string]bool)
	for _, item := range items {
		seen[item.Label] = true
	}

	// Extract symbols from document AST
	l := lexer.New(content)
	p := parser.New(l)
	prog := p.ParseProgram()
	if prog != nil {
		for _, stmt := range prog.Statements {
			switch s := stmt.(type) {
			case *ast.LetStatement:
				if s.Name != nil && !seen[s.Name.Value] {
					seen[s.Name.Value] = true
					kind := CompletionKindVariable
					if s.Constant {
						kind = CompletionKindConstant
					}
					items = append(items, CompletionItem{
						Label:  s.Name.Value,
						Kind:   kind,
						Detail: "Local definition",
					})
				}
			case *ast.EntityStatement:
				if s.Name != nil && !seen[s.Name.Value] {
					seen[s.Name.Value] = true
					items = append(items, CompletionItem{
						Label:  s.Name.Value,
						Kind:   CompletionKindStruct,
						Detail: "Entity schema",
					})
				}
			case *ast.ComponentLiteral:
				if s.Name != nil && !seen[s.Name.Value] {
					seen[s.Name.Value] = true
					items = append(items, CompletionItem{
						Label:  s.Name.Value,
						Kind:   CompletionKindClass,
						Detail: "Alap Component",
					})
				}
			case *ast.ExpressionStatement:
				if fn, ok := s.Expression.(*ast.FunctionLiteral); ok && fn.Name != "" {
					if !seen[fn.Name] {
						seen[fn.Name] = true
						items = append(items, CompletionItem{
							Label:  fn.Name,
							Kind:   CompletionKindFunction,
							Detail: "User function",
						})
					}
				}
			}
		}
	}

	return CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}
