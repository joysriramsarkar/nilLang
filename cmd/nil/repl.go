package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

const NIL_PROMPT = "নীলাং >> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()

	for {
		fmt.Printf(NIL_PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(out, "Scanner error: %v\n", err)
			}
			return
		}

		line := scanner.Text()
		if line == "exit" || line == "quit" {
			fmt.Println("নীলাং সেশন শেষ। Onuron OS-এর জন্য শুভকামনা!")
			return
		}

		// The REPL evaluates each line as a whole program, so it routes through
		// the same frontend gate as `nil run`. Definitions persist in env
		// because the evaluator reuses it across iterations.
		res := frontend.Check(line, frontend.Options{SkipTypeCheck: true})
		if len(res.ParseErrors) != 0 {
			printParserErrors(out, res.ParseErrors)
			continue
		}

		evaluated := evaluator.Eval(res.Program, env)
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
