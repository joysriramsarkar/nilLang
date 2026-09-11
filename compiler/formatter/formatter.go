package formatter

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// Format parses and pretty-prints Nilang source code into standard format
func Format(source string) (string, error) {
	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", fmt.Errorf("formatter parse error: %s", strings.Join(p.Errors(), "; "))
	}

	f := &ASTFormatter{indent: 0}
	return f.FormatProgram(prog), nil
}

type ASTFormatter struct {
	buf    bytes.Buffer
	indent int
}

func (f *ASTFormatter) writeIndent() {
	f.buf.WriteString(strings.Repeat("    ", f.indent))
}

func (f *ASTFormatter) FormatProgram(prog *ast.Program) string {
	for i, stmt := range prog.Statements {
		f.formatStatement(stmt)
		f.buf.WriteByte('\n')
		// Separate top-level function/component/entity definitions with an extra blank line
		if i < len(prog.Statements)-1 {
			switch stmt.(type) {
			case *ast.ComponentLiteral, *ast.EntityStatement:
				f.buf.WriteByte('\n')
			case *ast.ExpressionStatement:
				if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
					if _, isFn := exprStmt.Expression.(*ast.FunctionLiteral); isFn {
						f.buf.WriteByte('\n')
					}
				}
			}
		}
	}
	res := f.buf.String()
	// Ensure single trailing newline
	return strings.TrimRight(res, " \t\r\n") + "\n"
}

func (f *ASTFormatter) formatStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.LetStatement:
		f.writeIndent()
		kw := "let"
		if s.Constant {
			kw = "const"
		}
		typeAnn := ""
		if s.Type != "" {
			typeAnn = ": " + s.Type
		}
		val := ""
		if s.Value != nil {
			val = " = " + f.formatExpression(s.Value)
		}
		f.buf.WriteString(fmt.Sprintf("%s %s%s%s;", kw, s.Name.Value, typeAnn, val))

	case *ast.AssignStatement:
		f.writeIndent()
		op := "="
		if s.Operator != "" {
			op = s.Operator
		}
		f.buf.WriteString(fmt.Sprintf("%s %s %s;", s.Name.Value, op, f.formatExpression(s.Value)))

	case *ast.ReturnStatement:
		f.writeIndent()
		if s.ReturnValue != nil {
			f.buf.WriteString(fmt.Sprintf("return %s;", f.formatExpression(s.ReturnValue)))
		} else {
			f.buf.WriteString("return;")
		}

	case *ast.ExpressionStatement:
		f.writeIndent()
		exprStr := f.formatExpression(s.Expression)
		f.buf.WriteString(exprStr)
		// Don't append semicolon for blocks, if, while, functions
		if requiresSemicolon(s.Expression) {
			f.buf.WriteString(";")
		}

	case *ast.BlockStatement:
		f.formatBlock(s)

	case *ast.WhileStatement:
		f.writeIndent()
		f.buf.WriteString(fmt.Sprintf("while (%s) ", f.formatExpression(s.Condition)))
		f.formatBlock(s.Body)

	case *ast.StateDeclaration:
		f.writeIndent()
		typeAnn := ""
		if s.Type != "" {
			typeAnn = ": " + s.Type
		}
		val := ""
		if s.Value != nil {
			val = " = " + f.formatExpression(s.Value)
		}
		f.buf.WriteString(fmt.Sprintf("state %s%s%s;", s.Name.Value, typeAnn, val))

	case *ast.ImportStatement:
		f.writeIndent()
		if len(s.Names) > 0 {
			var names []string
			for _, n := range s.Names {
				names = append(names, n.Value)
			}
			f.buf.WriteString(fmt.Sprintf("import { %s } from %q;", strings.Join(names, ", "), s.Path.Value))
		} else if s.Alias != nil {
			f.buf.WriteString(fmt.Sprintf("import %q as %s;", s.Path.Value, s.Alias.Value))
		} else {
			f.buf.WriteString(fmt.Sprintf("import %q;", s.Path.Value))
		}

	case *ast.EntityStatement:
		f.writeIndent()
		f.buf.WriteString(fmt.Sprintf("entity %s {\n", s.Name.Value))
		f.indent++
		for _, field := range s.Fields {
			f.writeIndent()
			attrs := ""
			if field.IsPrimary {
				attrs += " primary"
			}
			if field.IsRequired {
				attrs += " required"
			}
			if field.IsUnique {
				attrs += " unique"
			}
			f.buf.WriteString(fmt.Sprintf("%s: %s%s;\n", field.Name, field.Type, attrs))
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")

	case *ast.ComponentLiteral:
		f.writeIndent()
		name := "Component"
		if s.Name != nil {
			name = s.Name.Value
		}
		f.buf.WriteString(fmt.Sprintf("component %s {\n", name))
		f.indent++
		for _, state := range s.States {
			f.formatStatement(state)
			f.buf.WriteByte('\n')
		}
		if s.Render != nil {
			f.writeIndent()
			f.buf.WriteString("render ")
			f.formatBlock(s.Render.Body)
			f.buf.WriteByte('\n')
		}
		if s.Build != nil {
			f.writeIndent()
			f.buf.WriteString("build ")
			f.formatBlock(s.Build.Body)
			f.buf.WriteByte('\n')
		}
		for _, handler := range s.Handlers {
			f.writeIndent()
			f.buf.WriteString(fmt.Sprintf("on %s ", handler.Event.Value))
			f.formatBlock(handler.Body)
			f.buf.WriteByte('\n')
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")

	case *ast.IndexAssignStatement:
		f.writeIndent()
		f.buf.WriteString(fmt.Sprintf("%s[%s] = %s;",
			f.formatExpression(s.Left),
			f.formatExpression(s.Index),
			f.formatExpression(s.Value)))

	case *ast.AppStatement:
		f.writeIndent()
		f.buf.WriteString("app")
		if s.Name != nil {
			f.buf.WriteString(" " + s.Name.Value)
		}
		f.buf.WriteString(" ")
		f.formatBlock(s.Body)

	case *ast.StyleStatement:
		f.writeIndent()
		name := ""
		if s.Name != nil {
			name = " " + s.Name.Value
		}
		f.buf.WriteString(fmt.Sprintf("style%s {\n", name))
		f.indent++
		var propKeys []string
		for k := range s.Properties {
			propKeys = append(propKeys, k)
		}
		sort.Strings(propKeys)
		for _, k := range propKeys {
			f.writeIndent()
			f.buf.WriteString(fmt.Sprintf("%s: %s;\n", k, s.Properties[k]))
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")

	default:
		f.writeIndent()
		f.buf.WriteString(stmt.String())
	}
}

func (f *ASTFormatter) formatBlock(block *ast.BlockStatement) {
	if block == nil {
		f.buf.WriteString("{\n}")
		return
	}
	f.buf.WriteString("{\n")
	f.indent++
	for _, s := range block.Statements {
		f.formatStatement(s)
		f.buf.WriteByte('\n')
	}
	f.indent--
	f.writeIndent()
	f.buf.WriteString("}")
}

func (f *ASTFormatter) formatBlockToString(block *ast.BlockStatement) string {
	oldBuf := f.buf
	f.buf = bytes.Buffer{}
	f.formatBlock(block)
	res := f.buf.String()
	f.buf = oldBuf
	return res
}

func (f *ASTFormatter) formatExpression(expr ast.Expression) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("%g", e.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.Boolean:
		return fmt.Sprintf("%t", e.Value)
	case *ast.NullLiteral:
		return "null"

	case *ast.PrefixExpression:
		return fmt.Sprintf("%s%s", e.Operator, f.formatExpression(e.Right))

	case *ast.InfixExpression:
		return fmt.Sprintf("%s %s %s", f.formatExpression(e.Left), e.Operator, f.formatExpression(e.Right))

	case *ast.CallExpression:
		var args []string
		for _, a := range e.Arguments {
			args = append(args, f.formatExpression(a))
		}
		return fmt.Sprintf("%s(%s)", f.formatExpression(e.Function), strings.Join(args, ", "))

	case *ast.ArrayLiteral:
		var elems []string
		for _, elem := range e.Elements {
			elems = append(elems, f.formatExpression(elem))
		}
		return fmt.Sprintf("[%s]", strings.Join(elems, ", "))

	case *ast.HashLiteral:
		var pairs []string
		for k, v := range e.Pairs {
			pairs = append(pairs, fmt.Sprintf("%s: %s", f.formatExpression(k), f.formatExpression(v)))
		}
		sort.Strings(pairs)
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))

	case *ast.IndexExpression:
		return fmt.Sprintf("%s[%s]", f.formatExpression(e.Left), f.formatExpression(e.Index))

	case *ast.DotExpression:
		return fmt.Sprintf("%s.%s", f.formatExpression(e.Left), e.Member.Value)

	case *ast.IfExpression:
		cond := f.formatExpression(e.Condition)
		conseq := f.formatBlockToString(e.Consequence)
		if e.Alternative != nil {
			alt := f.formatBlockToString(e.Alternative)
			return fmt.Sprintf("if (%s) %s else %s", cond, conseq, alt)
		}
		return fmt.Sprintf("if (%s) %s", cond, conseq)

	case *ast.FunctionLiteral:
		var params []string
		for _, p := range e.Parameters {
			params = append(params, p.Value)
		}
		name := ""
		if e.Name != "" {
			name = " " + e.Name
		}
		bodyStr := f.formatBlockToString(e.Body)
		return fmt.Sprintf("fn%s(%s) %s", name, strings.Join(params, ", "), bodyStr)
	}

	return expr.String()
}

func requiresSemicolon(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.IfExpression:
		return false
	case *ast.FunctionLiteral:
		return false
	default:
		return true
	}
}
