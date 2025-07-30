package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	dirPtr := flag.String("dir", ".", "The directory to analyze")
	outputPtr := flag.String("output", "output.txt", "The output file")
	flag.Parse()

	dir := *dirPtr

	outputFile := *outputPtr

	// Collect Go file structures
	var result strings.Builder
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Parse Go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fileContent, err := parseGoFile(path)
			if err != nil {
				return fmt.Errorf("error parsing file %s: %w", path, err)
			}
			relativePath := strings.TrimPrefix(path, dir)
			relativePath = strings.TrimPrefix(relativePath, "/")
			result.WriteString("file: " + relativePath + "\n")
			result.WriteString(fileContent + "\n")
		}

		return nil
	}); err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, []byte(result.String()), 0644); err != nil {
		fmt.Printf("Error writing to file %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Output written to %s\n", outputFile)
}

func parseGoFile(filePath string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)

	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("package " + node.Name.Name + "\n")
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			builder.WriteString("import " + x.Path.Value + "\n")
		case *ast.FuncDecl:
			builder.WriteString(formatFunctionSignature(fset, x) + "\n")
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					switch typeSpec.Type.(type) {
					case *ast.StructType:
						builder.WriteString(formatNode(fset, typeSpec) + "\n")
					case *ast.InterfaceType:
						builder.WriteString(formatNode(fset, typeSpec) + "\n")
					}
				}
			}
		}
		return true
	})

	return builder.String(), nil
}

func formatFunctionSignature(fset *token.FileSet, funcDecl *ast.FuncDecl) string {
	var builder strings.Builder

	builder.WriteString("func ")

	// Add receiver if it's a method
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		builder.WriteString("(")
		field := funcDecl.Recv.List[0]

		if len(field.Names) > 0 {
			builder.WriteString(field.Names[0].Name + " ")
		}

		builder.WriteString(formatNode(fset, field.Type))
		builder.WriteString(") ")
	}

	// Add function name
	builder.WriteString(funcDecl.Name.Name)
	function := formatNode(fset, funcDecl.Type)
	function = strings.Replace(function, "func", "", 1)
	builder.WriteString(function)

	return builder.String()
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	var buf strings.Builder
	printer.Fprint(&buf, fset, node)
	return buf.String()
}
