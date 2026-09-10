package lsp

import (
	"fmt"
	"strings"
	"unicode"
)

var keywordDocs = map[string]string{
	"let":       "**let** *identifier* [: *Type*] = *expression*;\n\nDeclares a mutable variable binding in the current lexical scope.",
	"const":     "**const** *identifier* [: *Type*] = *expression*;\n\nDeclares an immutable constant binding. Any reassignment produces compile error `E0104`.",
	"fn":        "**fn** [*name*](*param1*, *param2*, ...) [: *ReturnType*] { ... }\n\nDeclares a named function or anonymous closure.",
	"entity":    "**entity** *Name* { ... }\n\nDeclares a schema entity with typed fields, attributes (`primary`, `required`, `unique`), and ORM mapping.",
	"component": "**component** *Name* { ... }\n\nDeclares an Alap reactive UI component containing state, event handlers, and render tree.",
	"state":     "**state** *identifier* [: *Type*] = *initialValue*;\n\nDeclares a reactive state variable within an Alap component.",
	"render":    "**render** { ... }\n\nDefines the visual declaration tree of a component.",
	"build":     "**build** { ... }\n\nInitializes component tree and setup bindings before mounting.",
	"on":        "**on** *event* { ... }\n\nAttaches an event handler inside a component (e.g. `on click`, `on mount`).",
	"while":     "**while** (*condition*) { ... }\n\nIterative loop executed as long as condition evaluates to truthy.",
	"if":        "**if** (*condition*) { ... } [**else** { ... }]\n\nConditional branching expression or statement.",
	"else":      "**else** { ... }\n\nFallback branch for conditional `if`.",
	"return":    "**return** [*expression*];\n\nTerminates current function execution and yields return value.",
	"import":    "**import** { *symbols* } **from** *\"path\"*;\n\nImports exported declarations or modules.",
	"null":      "**null**\n\nRepresents the absence of any value or uninitialized reference.",
	"true":      "**true**\n\nBoolean literal representing logical truth.",
	"false":     "**false**\n\nBoolean literal representing logical false.",
}

var builtinDocs = map[string]string{
	"print":   "**print**(*values*...): *void*\n\nPrints arguments to standard output without trailing newline.",
	"println": "**println**(*values*...): *void*\n\nPrints arguments to standard output followed by a newline.",
	"puts":    "**puts**(*values*...): *void*\n\nOutputs string representation of arguments to standard output.",
	"len":     "**len**(*collection*): *Int*\n\nReturns the number of elements in an Array, String, or Hash.",
	"push":    "**push**(*array*, *element*): *Array*\n\nAppends element to array, returning the updated array.",
	"first":   "**first**(*array*): *Any*\n\nReturns the first element of array, or `null` if empty.",
	"last":    "**last**(*array*): *Any*\n\nReturns the last element of array, or `null` if empty.",
	"rest":    "**rest**(*array*): *Array*\n\nReturns a slice containing all elements of array except the first.",
	"type":    "**type**(*value*): *String*\n\nReturns the runtime type name of the specified value.",
	"assert":  "**assert**(*condition*, [*message*]): *void*\n\nAsserts condition is truthy; aborts with error if false.",
}

// GetWordAtPosition extracts the identifier/keyword token under (line, character).
func GetWordAtPosition(content string, line, character int) (string, Range) {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return "", Range{}
	}
	currentLine := lines[line]
	if character < 0 || character > len(currentLine) {
		return "", Range{}
	}

	start := character
	for start > 0 && isIdentChar(rune(currentLine[start-1])) {
		start--
	}
	end := character
	for end < len(currentLine) && isIdentChar(rune(currentLine[end])) {
		end++
	}

	if start >= end {
		return "", Range{}
	}

	word := currentLine[start:end]
	return word, Range{
		Start: Position{Line: line, Character: start},
		End:   Position{Line: line, Character: end},
	}
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// ComputeHover inspects document content at the given position and returns hover details.
func ComputeHover(content string, pos Position) *Hover {
	word, rng := GetWordAtPosition(content, pos.Line, pos.Character)
	if word == "" {
		return nil
	}

	if doc, ok := keywordDocs[word]; ok {
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: doc,
			},
			Range: &rng,
		}
	}

	if doc, ok := builtinDocs[word]; ok {
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: fmt.Sprintf("```nil\n%s\n```\n\n(Builtin Function)", doc),
			},
			Range: &rng,
		}
	}

	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: fmt.Sprintf("**%s**\n\n*Identifier*", word),
		},
		Range: &rng,
	}
}
