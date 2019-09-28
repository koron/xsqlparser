package sqltoken

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/akito0107/xsqlparser/dialect"
)

func TestTokenizer_Tokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  []*Token
	}{
		{
			name: "whitespace",
			in:   " ",
			out: []*Token{
				{
					Kind:  Whitespace,
					Value: " ",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
			},
		},
		{
			name: "whitespace and new line",
			in: `
 `,
			out: []*Token{
				{
					Kind:  Whitespace,
					Value: "\n",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 2, Col: 0},
				},
				{
					Kind:  Whitespace,
					Value: " ",
					From:  Pos{Line: 2, Col: 0},
					To:    Pos{Line: 2, Col: 1},
				},
			},
		},
		{
			name: "whitespace and tab",
			in: "\r\n	",
			out: []*Token{
				{
					Kind:  Whitespace,
					Value: "\n",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 2, Col: 0},
				},
				{
					Kind:  Whitespace,
					Value: "\t",
					From:  Pos{Line: 2, Col: 0},
					To:    Pos{Line: 2, Col: 4},
				},
			},
		},
		{
			name: "N string",
			in:   "N'string'",
			out: []*Token{
				{
					Kind:  NationalStringLiteral,
					Value: "string",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 9},
				},
			},
		},
		{
			name: "N string with keyword",
			in:   "N'string' NOT",
			out: []*Token{
				{
					Kind:  NationalStringLiteral,
					Value: "string",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 9},
				},
				{
					Kind:  Whitespace,
					Value: " ",
					From:  Pos{Line: 1, Col: 9},
					To:    Pos{Line: 1, Col: 10},
				},
				{
					Kind: SQLKeyword,
					Value: &SQLWord{
						Value:   "NOT",
						Keyword: "NOT",
					},
					From: Pos{Line: 1, Col: 10},
					To:   Pos{Line: 1, Col: 13},
				},
			},
		},
		{
			name: "Ident",
			in:   "select",
			out: []*Token{
				{
					Kind: SQLKeyword,
					Value: &SQLWord{
						Value:   "select",
						Keyword: "SELECT",
					},
					From: Pos{Line: 1, Col: 0},
					To:   Pos{Line: 1, Col: 6},
				},
			},
		},
		{
			name: "single quote string",
			in:   "'test'",
			out: []*Token{
				{
					Kind:  SingleQuotedString,
					Value: "test",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 6},
				},
			},
		},
		{
			name: "quoted string",
			in:   "\"SELECT\"",
			out: []*Token{
				{
					Kind: SQLKeyword,
					Value: &SQLWord{
						Value:      "SELECT",
						Keyword:    "SELECT",
						QuoteStyle: '"',
					},
					From: Pos{Line: 1, Col: 0},
					To:   Pos{Line: 1, Col: 8},
				},
			},
		},
		{
			name: "parents with number",
			in:   "(123),",
			out: []*Token{
				{
					Kind:  LParen,
					Value: "(",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  Number,
					Value: "123",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 4},
				},
				{
					Kind:  RParen,
					Value: ")",
					From:  Pos{Line: 1, Col: 4},
					To:    Pos{Line: 1, Col: 5},
				},
				{
					Kind:  Comma,
					Value: ",",
					From:  Pos{Line: 1, Col: 5},
					To:    Pos{Line: 1, Col: 6},
				},
			},
		},
		{
			name: "minus comment",
			in:   "-- test",
			out: []*Token{
				{
					Kind:  Comment,
					Value: " test",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 7},
				},
			},
		},
		{
			name: "minus operator",
			in:   "1-3",
			out: []*Token{
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  Minus,
					Value: "-",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 2},
				},
				{
					Kind:  Number,
					Value: "3",
					From:  Pos{Line: 1, Col: 2},
					To:    Pos{Line: 1, Col: 3},
				},
			},
		},
		{
			name: "/* comment",
			in: `/* test
multiline
comment */`,
			out: []*Token{
				{
					Kind:  Comment,
					Value: " test\nmultiline\ncomment ",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 3, Col: 10},
				},
			},
		},
		{
			name: "operators",
			in:   "1/1*1+1%1=1.1-.",
			out: []*Token{
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  Div,
					Value: "/",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 2},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 2},
					To:    Pos{Line: 1, Col: 3},
				},
				{
					Kind:  Mult,
					Value: "*",
					From:  Pos{Line: 1, Col: 3},
					To:    Pos{Line: 1, Col: 4},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 4},
					To:    Pos{Line: 1, Col: 5},
				},
				{
					Kind:  Plus,
					Value: "+",
					From:  Pos{Line: 1, Col: 5},
					To:    Pos{Line: 1, Col: 6},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 6},
					To:    Pos{Line: 1, Col: 7},
				},
				{
					Kind:  Mod,
					Value: "%",
					From:  Pos{Line: 1, Col: 7},
					To:    Pos{Line: 1, Col: 8},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 8},
					To:    Pos{Line: 1, Col: 9},
				},
				{
					Kind:  Eq,
					Value: "=",
					From:  Pos{Line: 1, Col: 9},
					To:    Pos{Line: 1, Col: 10},
				},
				{
					Kind:  Number,
					Value: "1.1",
					From:  Pos{Line: 1, Col: 10},
					To:    Pos{Line: 1, Col: 13},
				},
				{
					Kind:  Minus,
					Value: "-",
					From:  Pos{Line: 1, Col: 13},
					To:    Pos{Line: 1, Col: 14},
				},
				{
					Kind:  Period,
					Value: ".",
					From:  Pos{Line: 1, Col: 14},
					To:    Pos{Line: 1, Col: 15},
				},
			},
		},
		{
			name: "Neq",
			in:   "1!=2",
			out: []*Token{
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  Neq,
					Value: "!=",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 3},
				},
				{
					Kind:  Number,
					Value: "2",
					From:  Pos{Line: 1, Col: 3},
					To:    Pos{Line: 1, Col: 4},
				},
			},
		},
		{
			name: "Lts",
			in:   "<<=<>",
			out: []*Token{
				{
					Kind:  Lt,
					Value: "<",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  LtEq,
					Value: "<=",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 3},
				},
				{
					Kind:  Neq,
					Value: "<>",
					From:  Pos{Line: 1, Col: 3},
					To:    Pos{Line: 1, Col: 5},
				},
			},
		},
		{
			name: "Gts",
			in:   ">>=",
			out: []*Token{
				{
					Kind:  Gt,
					Value: ">",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  GtEq,
					Value: ">=",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 3},
				},
			},
		},
		{
			name: "colons",
			in:   ":1::1;",
			out: []*Token{
				{
					Kind:  Colon,
					Value: ":",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 2},
				},
				{
					Kind:  DoubleColon,
					Value: "::",
					From:  Pos{Line: 1, Col: 2},
					To:    Pos{Line: 1, Col: 4},
				},
				{
					Kind:  Number,
					Value: "1",
					From:  Pos{Line: 1, Col: 4},
					To:    Pos{Line: 1, Col: 5},
				},
				{
					Kind:  Semicolon,
					Value: ";",
					From:  Pos{Line: 1, Col: 5},
					To:    Pos{Line: 1, Col: 6},
				},
			},
		},
		{
			name: "others",
			in:   "\\[{&}]",
			out: []*Token{
				{
					Kind:  Backslash,
					Value: "\\",
					From:  Pos{Line: 1, Col: 0},
					To:    Pos{Line: 1, Col: 1},
				},
				{
					Kind:  LBracket,
					Value: "[",
					From:  Pos{Line: 1, Col: 1},
					To:    Pos{Line: 1, Col: 2},
				},
				{
					Kind:  LBrace,
					Value: "{",
					From:  Pos{Line: 1, Col: 2},
					To:    Pos{Line: 1, Col: 3},
				},
				{
					Kind:  Ampersand,
					Value: "&",
					From:  Pos{Line: 1, Col: 3},
					To:    Pos{Line: 1, Col: 4},
				},
				{
					Kind:  RBrace,
					Value: "}",
					From:  Pos{Line: 1, Col: 4},
					To:    Pos{Line: 1, Col: 5},
				},
				{
					Kind:  RBracket,
					Value: "]",
					From:  Pos{Line: 1, Col: 5},
					To:    Pos{Line: 1, Col: 6},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := strings.NewReader(c.in)
			tokenizer := NewTokenizer(src, &dialect.GenericSQLDialect{})

			tok, err := tokenizer.Tokenize()
			if err != nil {
				t.Errorf("should be no error %v", err)
			}

			if len(tok) != len(c.out) {
				t.Fatalf("should be same length but %d, %d", len(tok), len(c.out))
			}

			for i := 0; i < len(tok); i++ {
				if tok[i].Kind != c.out[i].Kind {
					t.Errorf("%d, expected sqltoken: %d, but got %d", i, c.out[i].Kind, tok[i].Kind)
				}
				if !reflect.DeepEqual(tok[i].Value, c.out[i].Value) {
					t.Errorf("%d, expected value: %+v, but got %+v", i, c.out[i].Value, tok[i].Value)
				}
				if !reflect.DeepEqual(tok[i].From, c.out[i].From) {
					t.Errorf("%d, expected value: %+v, but got %+v", i, c.out[i].From, tok[i].From)
				}
				if !reflect.DeepEqual(tok[i].To, c.out[i].To) {
					t.Errorf("%d, expected value: %+v, but got %+v", i, c.out[i].To, tok[i].To)
				}
			}
		})
	}
}

func TestTokenizer_Pos(t *testing.T) {
	t.Run("operators", func(t *testing.T) {
		cases := []struct {
			operator string
			add      int
		}{
			{
				operator: "+",
			},
			{
				operator: "-",
			},
			{
				operator: "%",
			},
			{
				operator: "*",
			},
			{
				operator: "/",
			},
			{
				operator: ">",
			},
			{
				operator: "=",
			},
			{
				operator: "<",
			},
			{
				operator: "<=",
				add:      1,
			},
			{
				operator: "<>",
				add:      1,
			},
			{
				operator: ">=",
				add:      1,
			},
		}

		for _, c := range cases {
			t.Run(c.operator, func(t *testing.T) {
				src := fmt.Sprintf("1 %s 1", c.operator)
				tokenizer := NewTokenizer(bytes.NewBufferString(src), &dialect.GenericSQLDialect{})

				if _, err := tokenizer.Tokenize(); err != nil {
					t.Fatal(err)
				}

				if d := cmp.Diff(tokenizer.Pos(), Pos{Line: 1, Col: 5 + c.add}); d != "" {
					t.Errorf("must be same but diff: %s", d)
				}
			})
		}
	})
	t.Run("other expressions", func(t *testing.T) {
		cases := []struct {
			name   string
			src    string
			expect Pos
		}{
			{
				name: "multiline ",
				src: `1+1
asdf`,
				expect: Pos{Line: 2, Col: 4},
			},
			{
				name:   "single line comment",
				src:    `-- comments`,
				expect: Pos{Line: 1, Col: 11},
			},
			{
				name:   "statements",
				src:    `select count(id) from account`,
				expect: Pos{Line: 1, Col: 29},
			},
			{
				name: "multiline statements",
				src: `select count(id)
from account 
where name like '%test%'`,
				expect: Pos{Line: 3, Col: 24},
			},
			{
				name: "multiline comment",
				src: `/*
test comment
test comment
*/`,
				expect: Pos{Line: 4, Col: 2},
			},
			{
				name:   "single line comment",
				src:    "/* asdf */",
				expect: Pos{Line: 1, Col: 10},
			},
			{
				name:   "comment inside sql",
				src:    "select * from /* test table */ test_table where id != 123",
				expect: Pos{Line: 1, Col: 57},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				tokenizer := NewTokenizer(bytes.NewBufferString(c.src), &dialect.GenericSQLDialect{})

				if _, err := tokenizer.Tokenize(); err != nil {
					t.Fatal(err)
				}

				if d := cmp.Diff(tokenizer.Pos(), c.expect); d != "" {
					t.Errorf("must be same but diff: %s", d)
				}
			})
		}
	})

}