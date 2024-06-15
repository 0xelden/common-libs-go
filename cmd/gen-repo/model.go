package main

type Table struct {
	Schema  string
	Name    string
	Columns []Column
}

type Column struct {
	Name                 string
	RawType              string
	NotNull              bool
	IsPrimary            bool
	HasDefault           bool
	IsGenerated          bool
	GenerationExpression string
}
