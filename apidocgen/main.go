/*
 * Code taken and changed from https://github.com/oracle/coherence-operator/blob/v3.2.5/docgen/main.go
 *
 * Copyright (c) 2020 Oracle and/or its affiliates.
 * Licensed under the Universal Permissive License v 1.0 as shown at
 * http://oss.oracle.com/licenses/upl.
 */

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

const (
	firstParagraph = `
= Hazelcast Platform Operator API Docs

A reference guide to the Hazelcast Platform Operator CRD types.

== Hazelcast Platform Operator API Docs

This is a reference for the Hazelcast Platform Operator API types.
These are all the types and fields that are used in the Hazelcast Platform Operator CRDs. 

TIP: This document was generated from comments in the Go code in the api/ directory.`

	k8sLink = "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#"
)

type Field struct {
	Name, Doc, Type, Default string
	Mandatory                bool
}

type StructType struct {
	Name, Doc string
	Fields    []Field
}

type Const struct {
	Name, Doc, Type, Value string
}

type StringType struct {
	Name, Doc string
	Consts    []Const
}

var (
	selfLinks = map[string]string{}
)

func main() {
	printAPIDocs(os.Args[1:])
}

func printAPIDocs(paths []string) {
	fmt.Println(firstParagraph)

	allTypes := getAllTypes(paths)
	for _, t := range allTypes {
		selfLinks[t.Name] = fmt.Sprintf("<<%s,%s>>", t.Name, t.Name)
	}

	printContentTable(allTypes)

	printStructs(parseStructDocumentation(paths))
	printStrings(parseStringTypeDocumentation(paths))
}

func getAllTypes(srcs []string) []*doc.Type {
	var docForTypes []*doc.Type

	for _, src := range srcs {
		pkg := astFrom(src)
		docForTypes = append(docForTypes, pkg.Types...)
	}

	return docForTypes
}

func parseStructDocumentation(srcs []string) []StructType {
	var docForTypes []StructType

	for _, src := range srcs {
		pkg := astFrom(src)
		for _, kubType := range pkg.Types {
			if structType, ok := kubType.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
				ks := processFields(structType, []Field{})
				st := StructType{Name: kubType.Name, Doc: fmtRawDoc(kubType.Doc), Fields: ks}
				docForTypes = append(docForTypes, st)
			}
		}
	}
	return docForTypes
}

func parseStringTypeDocumentation(srcs []string) []StringType {
	var stTypes []StringType
	for _, src := range srcs {
		pkg := astFrom(src)
		for _, kubType := range pkg.Types {
			if stringType, ok := kubType.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.Ident); !ok || stringType.Name != "string" {
				continue
			}
			var cs []Const
			for _, kubTypeConst := range kubType.Consts {

				for _, consPec := range kubTypeConst.Decl.Specs {
					cons, ok := consPec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					// fmt.Printf("Doc for cons.Doc.Text() is %v\n", cons.Doc.Text())
					bslit, ok := cons.Values[0].(*ast.BasicLit)
					if !ok {
						continue
					}
					if cons.Type.(*ast.Ident).Name != kubType.Name {
						panic("Const type must be equal to its generated one.")
					}
					// fmt.Printf("Const type is %v, name is %v and val is %v\n", cons.Type.(*ast.Ident).Name, cons.Names[0], bslit.Value)
					cs = append(cs, Const{Name: cons.Names[0].Name,
						Doc:   fmtRawDoc(cons.Doc.Text()),
						Type:  cons.Type.(*ast.Ident).Name,
						Value: bslit.Value,
					})
				}
			}
			stTypes = append(stTypes, StringType{Name: kubType.Name, Doc: fmtRawDoc(kubType.Doc), Consts: cs})
		}
	}
	return stTypes
}

func astFrom(filePath string) *doc.Package {
	fset := token.NewFileSet()
	m := make(map[string]*ast.File)

	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	m[filePath] = f
	apkg, _ := ast.NewPackage(fset, m, nil, nil)

	return doc.New(apkg, "", 0)
}

func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	delPrevChar := func() {
		if buffer.Len() > 0 {
			buffer.Truncate(buffer.Len() - 1) // Delete the last " " or "\n"
		}
	}

	// Ignore all lines after ---
	rawDoc = strings.Split(rawDoc, "---")[0]

	for _, line := range strings.Split(rawDoc, "\n") {
		line = strings.Trim(line, " ")
		switch {
		case len(line) == 0: // Keep paragraphs
			delPrevChar()
			buffer.WriteString("\n\n")
		case strings.HasPrefix(line, "TODO"): // Ignore one line TODOs
		case strings.HasPrefix(line, "+"): // Ignore instructions to go2idl
		default:
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				delPrevChar()
				line = "\n" + line + " +\n" // Replace it with newline. This is useful when we have a line with: "Example:\n\tJSON-someting..."
			} else {
				line += " "
			}
			buffer.WriteString(line)
		}
	}

	postDoc := strings.TrimRight(buffer.String(), "\n")
	postDoc = strings.Replace(postDoc, "\\\"", "\"", -1) // replace user's \" to "
	// postDoc = strings.Replace(postDoc, "\"", "\\\"", -1) // Escape "
	postDoc = strings.Replace(postDoc, "\n", " +\n", -1)
	postDoc = strings.Replace(postDoc, "\t", "&#160;&#160;&#160;&#160;", -1)
	postDoc = strings.Replace(postDoc, "|", "\\|", -1)
	return postDoc
}

func processFields(structType *ast.StructType, ks []Field) []Field {
	for _, field := range structType.Fields.List {
		typeString := fieldType(field.Type)
		fieldMandatory := fieldRequired(field)
		defaultValue := fieldDefault(field)
		if n := fieldName(field); n != "-" {
			fieldDoc := fmtRawDoc(field.Doc.Text())
			ks = append(ks,
				Field{Name: n,
					Doc:       fieldDoc,
					Type:      typeString,
					Default:   defaultValue,
					Mandatory: fieldMandatory})
		} else if strings.Contains(field.Tag.Value, "json:\",inline") {
			// TODO Try to find structs of selector expressions.
			if ident, ok := field.Type.(*ast.Ident); ok && ident.Obj != nil {
				if ts, ok := ident.Obj.Decl.(*ast.TypeSpec); ok {
					if st, ok := ts.Type.(*ast.StructType); ok {
						ks = processFields(st, ks)
					}
				}
			}
		}
	}
	return ks
}

func fieldType(typ ast.Expr) string {
	switch t := typ.(type) {
	case *ast.Ident:
		return toLink(t.Name)
	case *ast.StarExpr:
		return "&#42;" + toLink(fieldType(typ.(*ast.StarExpr).X))
	case *ast.SelectorExpr:
		e := typ.(*ast.SelectorExpr)
		pkg := e.X.(*ast.Ident)
		return toLink(pkg.Name + "." + e.Sel.Name)
	case *ast.ArrayType:
		return "[]" + toLink(fieldType(typ.(*ast.ArrayType).Elt))
	case *ast.MapType:
		mapType := typ.(*ast.MapType)
		return "map[" + toLink(fieldType(mapType.Key)) + "]" + toLink(fieldType(mapType.Value))
	default:
		return ""
	}
}

func toLink(typeName string) string {
	selfLink, hasSelfLink := selfLinks[typeName]
	if hasSelfLink {
		return selfLink
	}

	switch {
	case strings.HasPrefix(typeName, "corev1."):
		return fmt.Sprintf("%s%s-v1-core[%s]", k8sLink, strings.ToLower(typeName[7:]), escapeTypeName(typeName))
	case strings.HasPrefix(typeName, "metav1."):
		return fmt.Sprintf("%s%s-v1-meta[%s]", k8sLink, strings.ToLower(typeName[7:]), escapeTypeName(typeName))
	}

	return typeName
}

func escapeTypeName(typeName string) string {
	if strings.HasPrefix(typeName, "*") {
		return "&#42;" + typeName[1:]
	}
	return typeName
}

// fieldRequired returns whether a field is a required field.
// If omitempty json tag or +optional kubebuilder tags is there, the field is optional. Otherwise, required.
func fieldRequired(field *ast.Field) bool {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json") // Delete first and last quotation
		if strings.Contains(jsonTag, "omitempty") {
			return false
		}
	}

	doc := field.Doc.Text()
	for _, line := range strings.Split(doc, "\n") {
		line = strings.Trim(line, " ")
		if line == "+optional" {
			return false
		}
	}
	return true
}

func fieldDefault(field *ast.Field) string {
	doc := field.Doc.Text()
	for _, line := range strings.Split(doc, "\n") {
		line = strings.Trim(line, " ")
		if strings.HasPrefix(line, "+kubebuilder:default:=") {
			return strings.Split(line, ":=")[1]
		}
	}
	return "-"
}

// fieldName returns the name of the field as it should appear in JSON format
// "-" indicates that this field is not part of the JSON representation
func fieldName(field *ast.Field) string {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json") // Delete first and last quotation
		if strings.Contains(jsonTag, "inline") {
			return "-"
		}
	}

	jsonTag = strings.Split(jsonTag, ",")[0] // This can return "-"
	if jsonTag == "" {
		if field.Names != nil {
			return field.Names[0].Name
		}
		return field.Type.(*ast.Ident).Name
	}
	return jsonTag
}

func printContentTable(types []*doc.Type) {
	fmt.Printf("\n=== Table of Contents\n")
	for _, t := range types {
		sn := t.Name
		// if len(t.Fields) > 0 {
		fmt.Printf("* <<%s,%s>>\n", sn, sn)
		// }
	}
}

func printStructs(structTypes []StructType) {
	for _, t := range structTypes {
		if len(t.Fields) > 0 {
			fmt.Printf("\n=== %s\n\n%s\n\n", t.Name, t.Doc)

			fmt.Println("[cols=\"4,8,4,2,4\"options=\"header\"]")
			fmt.Println("|===")
			fmt.Println("| Field | Description | Type | Required | Default")
			for _, f := range t.Fields {
				var d string
				if f.Doc == "" {
					d = "&#160;"
				} else {
					d = strings.ReplaceAll(f.Doc, "\\n", " +\n")
				}
				fmt.Println("m|", f.Name, "|", d, "m|", f.Type, "|", f.Mandatory, "|", f.Default)
			}
			fmt.Println("|===")
			fmt.Println("")
			fmt.Println("<<Table of Contents,Back to TOC>>")
		}
	}

}

func printStrings(stringTypes []StringType) {
	for _, t := range stringTypes {
		fmt.Printf("\n=== %s\n\n%s\n\n", t.Name, t.Doc)

		fmt.Println("[cols=\"5,10\"options=\"header\"]")
		fmt.Println("|===")
		fmt.Println("| Value | Description")
		for _, f := range t.Consts {
			var d string
			if f.Doc == "" {
				d = "&#160;"
			} else {
				d = strings.ReplaceAll(f.Doc, "\\n", " +\n")
			}
			fmt.Println("m|", f.Value, "|", d)
		}
		fmt.Println("|===")
		fmt.Println("")
		fmt.Println("<<Table of Contents,Back to TOC>>")
	}
}
