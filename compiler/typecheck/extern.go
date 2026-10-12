package typecheck

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// checkExternDecl registers an `extern "C" fn` so that its call sites type-check.
//
// # Why a declaration is type-checked like a definition
//
// The signature is the only thing the language knows about the function, so it is
// the only thing the language can check a call against. Registering it with the
// annotations exactly as written means a call passing the wrong types is a
// diagnostic here — a line number and a message — rather than a conversion failure
// inside the FFI at run time, which is a corrupted call and a stack nobody can
// read back.
//
// # Why an unknown type in the signature is reported rather than accepted
//
// `types.Parse` succeeds for any name, so a typo arrives as an empty struct type
// rather than an error. Every parameter would then accept any value and the return
// would be an empty struct, which is a declaration that type-checks and means
// nothing. The same rule the rest of the checker applies to a parameter annotation
// applies here: a type that does not resolve is a diagnostic.
//
// # Why the arity is exact
//
// A C function takes a fixed number of arguments. The language's own functions may
// take a range, and a call with the wrong count would be a compile-time error at
// the call; pinning `MinArgs` and `MaxArgs` to the declared count means the error
// arrives there too, naming the argument list instead of arriving as a call
// reading a register the callee never expected to be set.
func (c *Checker) checkExternDecl(decl *ast.ExternStatement) {
	if decl.Fn == nil || decl.Fn.Name == "" {
		// The parser already rejects this. Checking it here as well would mean two
		// diagnostics for one mistake, and the second would arrive later and say
		// less.
		return
	}

	paramTypes := make([]types.Type, 0, len(decl.Fn.Parameters))
	for _, p := range decl.Fn.Parameters {
		if p == nil || p.Name == nil {
			continue
		}
		pt := types.Type(types.Any)
		if p.TypeAnnotation != "" {
			resolved, err := c.resolveType(p.TypeAnnotation)
			switch {
			case err != nil || !c.isKnownTypeName(p.TypeAnnotation):
				c.report(CodeUnknownType,
					fmt.Sprintf("unknown type %q for parameter %q of extern %q",
						p.TypeAnnotation, p.Name.Value, decl.Fn.Name),
					p.Token.Line, p.Token.Column)
			default:
				pt = resolved
			}
		}
		paramTypes = append(paramTypes, pt)
	}

	returnType := types.Type(types.Any)
	if decl.Fn.ReturnType != "" {
		resolved, err := c.resolveType(decl.Fn.ReturnType)
		switch {
		case err != nil || !c.isKnownTypeName(decl.Fn.ReturnType):
			c.report(CodeUnknownType,
				fmt.Sprintf("unknown type %q returned by extern %q",
					decl.Fn.ReturnType, decl.Fn.Name),
				decl.Fn.Token.Line, decl.Fn.Token.Column)
		default:
			returnType = resolved
		}
	}

	count := len(paramTypes)
	c.currentScope.SetVar(decl.Fn.Name, &types.FunctionType{
		Params:     paramTypes,
		ReturnType: returnType,
		MinArgs:    count,
		MaxArgs:    count,
	})
}
