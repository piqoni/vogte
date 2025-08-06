package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
	"golang.org/x/mod/modfile"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) ParseProject(dir string) (string, error) {
	var result strings.Builder
	modulePath := ""
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			fileContent, err := p.parseProtoFile(path)
			if err != nil {
				return fmt.Errorf("error parsing file %s: %w", path, err)
			}
			relativePath := strings.TrimPrefix(path, dir)
			relativePath = strings.TrimPrefix(relativePath, "/")
			result.WriteString("file: " + relativePath + "\n")
			result.WriteString(fileContent + "\n")
		}

		if !info.IsDir() && strings.HasSuffix(path, "go.mod") {
			modulePath = p.getModulePath(path)
		}

		// Parse Go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fileContent, err := p.parseGoFile(path, modulePath)
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

	return result.String(), nil
}

func (p *Parser) parseGoFile(filePath, modulePath string) (string, error) {
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
			// Collect only internal imports for now
			if modulePath != "" && strings.Contains(x.Path.Value, modulePath) {
				builder.WriteString("import " + x.Path.Value + "\n")
			}
		case *ast.FuncDecl:
			builder.WriteString(p.formatFunctionSignature(fset, x) + "\n")
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					switch typeSpec.Type.(type) {
					case *ast.StructType:
						builder.WriteString(p.formatNode(fset, typeSpec) + "\n")
					case *ast.InterfaceType:
						builder.WriteString(p.formatNode(fset, typeSpec) + "\n")
					}
				}
			}
		}
		return true
	})

	return builder.String(), nil
}

func (p *Parser) getModulePath(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	modFile, err := modfile.Parse(filePath, data, nil)
	if err != nil {
		return ""
	}

	return modFile.Module.Mod.Path
}

// Extract from proto files messages and services names
func (p *Parser) parseProtoFile(filepath string) (string, error) {
	reader, _ := os.Open(filepath)
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, _ := parser.Parse()

	var content []string
	proto.Walk(definition,
		proto.WithMessage(func(m *proto.Message) {
			content = append(content, "message "+m.Name)
		}),
		proto.WithService(func(s *proto.Service) {
			content = append(content, "service "+s.Name)
		}),
	)

	result := strings.Join(content, "\n")
	return result, nil
}

func (p *Parser) formatFunctionSignature(fset *token.FileSet, funcDecl *ast.FuncDecl) string {
	var builder strings.Builder

	builder.WriteString("func ")

	// Add receiver if it's a method
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		builder.WriteString("(")
		field := funcDecl.Recv.List[0]

		if len(field.Names) > 0 {
			builder.WriteString(field.Names[0].Name + " ")
		}

		builder.WriteString(p.formatNode(fset, field.Type))
		builder.WriteString(") ")
	}

	// Add function name
	builder.WriteString(funcDecl.Name.Name)
	function := p.formatNode(fset, funcDecl.Type)
	function = strings.Replace(function, "func", "", 1)
	builder.WriteString(function)

	return builder.String()
}

func (p *Parser) formatNode(fset *token.FileSet, node ast.Node) string {
	var buf strings.Builder
	printer.Fprint(&buf, fset, node)
	return buf.String()
}
