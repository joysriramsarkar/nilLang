package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

const NIL_PROMPT = "নীলাং >> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

	for {
		fmt.Printf(NIL_PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		if line == "exit" || line == "quit" {
			fmt.Println("নীলাং সেশন শেষ। Onuron OS-এর জন্য শুভকামনা!")
			return
		}

		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		evaluated := evaluator.Eval(program, env)
		if evaluated != nil {
			io.WriteString(out, evaluated.Inspect())
			io.WriteString(out, "\n")
		}
	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "Oops! সিনট্যাক্স এরর:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func cmdRepl() {
	fmt.Println("স্বাগতম Nilang (নীলাং) REPL-এ!")
	fmt.Println("Alap Framework এবং Onuron OS-এর পাওয়ার্ড বাই।")
	fmt.Println("বন্ধ করতে 'exit' বা 'quit' টাইপ করুন।")
	Start(os.Stdin, os.Stdout)
}