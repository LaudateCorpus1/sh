// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package sh

import (
	"reflect"
	"strings"
	"testing"
)

func lit(s string) Lit { return Lit{Value: s} }
func lits(strs ...string) []Node {
	l := make([]Node, len(strs))
	for i, s := range strs {
		l[i] = lit(s)
	}
	return l
}

func word(ns ...Node) Word  { return Word{Parts: ns} }
func litWord(s string) Word { return word(lits(s)...) }
func litWords(strs ...string) []Word {
	l := make([]Word, 0, len(strs))
	for _, s := range strs {
		l = append(l, litWord(s))
	}
	return l
}

func litCmd(strs ...string) Command {
	return Command{Args: litWords(strs...)}
}

func stmt(n Node) Stmt { return Stmt{Node: n} }
func stmts(ns ...Node) []Stmt {
	l := make([]Stmt, len(ns))
	for i, n := range ns {
		l[i] = stmt(n)
	}
	return l
}

func litStmt(strs ...string) Stmt { return stmt(litCmd(strs...)) }
func litStmts(strs ...string) []Stmt {
	l := make([]Stmt, len(strs))
	for i, s := range strs {
		l[i] = litStmt(s)
	}
	return l
}

func dblQuoted(ns ...Node) DblQuoted  { return DblQuoted{Parts: ns} }
func bckQuoted(sts ...Stmt) BckQuoted { return BckQuoted{Stmts: sts} }
func block(sts ...Stmt) Block         { return Block{Stmts: sts} }
func cmdSubst(sts ...Stmt) CmdSubst   { return CmdSubst{Stmts: sts} }

type testCase struct {
	strs []string
	ast  interface{}
}

var tests = []testCase{
	{
		[]string{"", " ", "\t", "\n \n"},
		nil,
	},
	{
		[]string{"", "# foo", "# foo ( bar", "# foo'bar"},
		nil,
	},
	{
		[]string{"foo", "foo ", " foo", "foo # bar"},
		litCmd("foo"),
	},
	{
		[]string{"foo; bar", "foo; bar;", "foo;bar;", "\nfoo\nbar\n"},
		litStmts("foo", "bar"),
	},
	{
		[]string{"foo a b", " foo  a  b ", "foo \\\n a b"},
		litCmd("foo", "a", "b"),
	},
	{
		[]string{"foobar", "foo\\\nbar"},
		litCmd("foobar"),
	},
	{
		[]string{"foo'bar'"},
		litCmd("foo'bar'"),
	},
	{
		[]string{"(foo)", "(foo;)", "(\nfoo\n)"},
		Subshell{Stmts: litStmts("foo")},
	},
	{
		[]string{"{ foo; }", "{foo;}", "{\nfoo\n}"},
		block(litStmt("foo")),
	},
	{
		[]string{
			"if a; then b; fi",
			"if a\nthen\nb\nfi",
		},
		IfStmt{
			Conds:     litStmts("a"),
			ThenStmts: litStmts("b"),
		},
	},
	{
		[]string{
			"if a; then b; else c; fi",
			"if a\nthen b\nelse\nc\nfi",
		},
		IfStmt{
			Conds:     litStmts("a"),
			ThenStmts: litStmts("b"),
			ElseStmts: litStmts("c"),
		},
	},
	{
		[]string{
			"if a; then a; elif b; then b; elif c; then c; else d; fi",
			"if a\nthen a\nelif b\nthen b\nelif c\nthen c\nelse\nd\nfi",
		},
		IfStmt{
			Conds:     litStmts("a"),
			ThenStmts: litStmts("a"),
			Elifs: []Elif{
				{
					Conds:     litStmts("b"),
					ThenStmts: litStmts("b"),
				},
				{
					Conds:     litStmts("c"),
					ThenStmts: litStmts("c"),
				},
			},
			ElseStmts: litStmts("d"),
		},
	},
	{
		[]string{"if a1; a2 foo; a3 bar; then b; fi"},
		IfStmt{
			Conds: []Stmt{
				litStmt("a1"),
				litStmt("a2", "foo"),
				litStmt("a3", "bar"),
			},
			ThenStmts: litStmts("b"),
		},
	},
	{
		[]string{"while a; do b; done", "while a\ndo\nb\ndone"},
		WhileStmt{
			Conds:   litStmts("a"),
			DoStmts: litStmts("b"),
		},
	},
	{
		[]string{
			"for i; do foo; done",
			"for i in; do foo; done",
		},
		ForStmt{
			Name:    lit("i"),
			DoStmts: litStmts("foo"),
		},
	},
	{
		[]string{
			"for i in 1 2 3; do echo $i; done",
			"for i in 1 2 3\ndo echo $i\ndone",
		},
		ForStmt{
			Name:     lit("i"),
			WordList: litWords("1", "2", "3"),
			DoStmts: stmts(Command{Args: []Word{
				litWord("echo"),
				word(ParamExp{Short: true, Text: "i"}),
			}}),
		},
	},
	{
		[]string{`echo ' ' "foo bar"`},
		Command{Args: []Word{
			litWord("echo"),
			litWord("' '"),
			word(dblQuoted(lits("foo bar")...)),
		}},
	},
	{
		[]string{`"foo \" bar"`},
		Command{Args: []Word{
			word(dblQuoted(lits(`foo \" bar`)...)),
		}},
	},
	{
		[]string{"\">foo\" \"\nbar\""},
		Command{Args: []Word{
			word(dblQuoted(lit(">foo"))),
			word(dblQuoted(lit("\nbar"))),
		}},
	},
	{
		[]string{`foo \" bar`},
		litCmd(`foo`, `\"`, `bar`),
	},
	{
		[]string{"s{s s=s"},
		litCmd("s{s", "s=s"),
	},
	{
		[]string{"foo && bar", "foo&&bar", "foo &&\nbar"},
		BinaryExpr{
			Op: LAND,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"foo || bar", "foo||bar", "foo ||\nbar"},
		BinaryExpr{
			Op: LOR,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"if a; then b; fi || while a; do b; done"},
		BinaryExpr{
			Op: LOR,
			X: stmt(IfStmt{
				Conds:     litStmts("a"),
				ThenStmts: litStmts("b"),
			}),
			Y: stmt(WhileStmt{
				Conds:   litStmts("a"),
				DoStmts: litStmts("b"),
			}),
		},
	},
	{
		[]string{"foo && bar1 || bar2"},
		BinaryExpr{
			Op: LAND,
			X:  litStmt("foo"),
			Y: stmt(BinaryExpr{
				Op: LOR,
				X:  litStmt("bar1"),
				Y:  litStmt("bar2"),
			}),
		},
	},
	{
		[]string{"foo | bar", "foo|bar"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y:  litStmt("bar"),
		},
	},
	{
		[]string{"foo | bar | extra"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y: stmt(BinaryExpr{
				Op: OR,
				X:  litStmt("bar"),
				Y:  litStmt("extra"),
			}),
		},
	},
	{
		[]string{
			"foo() { a; b; }",
			"foo() {\na\nb\n}",
			"foo ( ) {\na\nb\n}",
		},
		FuncDecl{
			Name: lit("foo"),
			Body: stmt(block(litStmts("a", "b")...)),
		},
	},
	{
		[]string{"foo() { a; }; bar", "foo() {\na\n}\nbar"},
		[]Node{
			FuncDecl{
				Name: lit("foo"),
				Body: stmt(block(litStmts("a")...)),
			},
			litCmd("bar"),
		},
	},
	{
		[]string{
			"foo >a >>b <c",
			"foo > a >> b < c",
			">a >>b foo <c",
		},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("a")},
				{Op: APPEND, Word: litWord("b")},
				{Op: RDRIN, Word: litWord("c")},
			},
		},
	},
	{
		[]string{
			"foo bar >a",
			"foo >a bar",
		},
		Stmt{
			Node: litCmd("foo", "bar"),
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("a")},
			},
		},
	},
	{
		[]string{`>a >\b`},
		Stmt{
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("a")},
				{Op: RDROUT, Word: litWord(`\b`)},
			},
		},
	},
	{
		[]string{
			"foo <<EOF\nbar\nEOF",
			"foo <<EOF\nbar",
		},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: HEREDOC, Word: litWord("EOF\nbar\nEOF")},
			},
		},
	},
	{
		[]string{"foo <<FOOBAR\nbar\nFOOBAR"},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: HEREDOC, Word: litWord("FOOBAR\nbar\nFOOBAR")},
			},
		},
	},
	{
		[]string{
			"foo <<-EOF\nbar\nEOF",
			"foo <<- EOF\nbar\nEOF",
		},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: DHEREDOC, Word: litWord("EOF\nbar\nEOF")},
			},
		},
	},
	{
		[]string{"foo >&2 <&0 2>file <>f2"},
		Stmt{
			Node: litCmd("foo"),
			Redirs: []Redirect{
				{Op: DPLOUT, Word: litWord("2")},
				{Op: DPLIN, Word: litWord("0")},
				{Op: RDROUT, N: lit("2"), Word: litWord("file")},
				{Op: OPRDWR, Word: litWord("f2")},
			},
		},
	},
	{
		[]string{"a >f1; b >f2"},
		[]Stmt{
			{
				Node:   litCmd("a"),
				Redirs: []Redirect{{Op: RDROUT, Word: litWord("f1")}},
			},
			{
				Node:   litCmd("b"),
				Redirs: []Redirect{{Op: RDROUT, Word: litWord("f2")}},
			},
		},
	},
	{
		[]string{"foo &", "foo&"},
		Stmt{
			Node:       litCmd("foo"),
			Background: true,
		},
	},
	{
		[]string{"foo &; bar", "foo & bar", "foo&bar"},
		[]Stmt{
			{
				Node:       litCmd("foo"),
				Background: true,
			},
			litStmt("bar"),
		},
	},
	{
		[]string{"if foo; then bar; fi >/dev/null &"},
		Stmt{
			Node: IfStmt{
				Conds:     litStmts("foo"),
				ThenStmts: litStmts("bar"),
			},
			Redirs: []Redirect{
				{Op: RDROUT, Word: litWord("/dev/null")},
			},
			Background: true,
		},
	},
	{
		[]string{"echo foo#bar"},
		litCmd("echo", "foo#bar"),
	},
	{
		[]string{"$(foo bar)"},
		Command{Args: []Word{
			word(cmdSubst(litStmt("foo", "bar"))),
		}},
	},
	{
		[]string{"$(foo | bar)"},
		Command{Args: []Word{
			word(cmdSubst(stmt(BinaryExpr{
				Op: OR,
				X:  litStmt("foo"),
				Y:  litStmt("bar"),
			}))),
		}},
	},
	{
		[]string{"`foo`"},
		Command{Args: []Word{
			word(bckQuoted(litStmt("foo"))),
		}},
	},
	{
		[]string{"`foo | bar`"},
		Command{Args: []Word{
			word(bckQuoted(stmt(BinaryExpr{
				Op: OR,
				X:  litStmt("foo"),
				Y:  litStmt("bar"),
			}))),
		}},
	},
	{
		[]string{"`foo 'bar'`"},
		Command{Args: []Word{
			word(bckQuoted(litStmt("foo", "'bar'"))),
		}},
	},
	{
		[]string{"`foo \"bar\"`"},
		Command{Args: []Word{
			word(bckQuoted(stmt(Command{Args: []Word{
				litWord("foo"),
				word(dblQuoted(lit("bar"))),
			}}))),
		}},
	},
	{
		[]string{`echo "$foo"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(ParamExp{Short: true, Text: "foo"})),
		}},
	},
	{
		[]string{`$@ $# $$`},
		Command{Args: []Word{
			word(ParamExp{Short: true, Text: "@"}),
			word(ParamExp{Short: true, Text: "#"}),
			word(ParamExp{Short: true, Text: "$"}),
		}},
	},
	{
		[]string{`echo "${foo}"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(ParamExp{Text: "foo"})),
		}},
	},
	{
		[]string{`echo "(foo)"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(lit("(foo)"))),
		}},
	},
	{
		[]string{`echo "$(foo)"`},
		Command{Args: []Word{
			litWord("echo"),
			word(dblQuoted(cmdSubst(litStmt("foo")))),
		}},
	},
	{
		[]string{`echo '${foo}'`},
		litCmd("echo", "'${foo}'"),
	},
	{
		[]string{"echo ${foo bar}"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Text: "foo bar"}),
		}},
	},
	{
		[]string{"$(($x-1))"},
		Command{Args: []Word{word(ArithmExp{Text: "$x-1"})}},
	},
	{
		[]string{"echo foo$bar"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), ParamExp{Short: true, Text: "bar"}),
		}},
	},
	{
		[]string{"echo foo$(bar)"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), cmdSubst(litStmt("bar"))),
		}},
	},
	{
		[]string{"echo foo${bar bar}"},
		Command{Args: []Word{
			litWord("echo"),
			word(lit("foo"), ParamExp{Text: "bar bar"}),
		}},
	},
	{
		[]string{"echo 'foo${bar'"},
		litCmd("echo", "'foo${bar'"),
	},
	{
		[]string{"(foo); bar"},
		[]Node{
			Subshell{Stmts: litStmts("foo")},
			litCmd("bar"),
		},
	},
	{
		[]string{"a=\"\nbar\""},
		Command{Args: []Word{
			word(lit("a="), dblQuoted(lit("\nbar"))),
		}},
	},
	{
		[]string{
			"case $i in 1) foo;; 2 | 3*) bar; esac",
			"case $i in 1) foo;; 2 | 3*) bar;; esac",
			"case $i\nin\n1)\nfoo\n;;\n2 | 3*)\nbar\n;;\nesac",
		},
		CaseStmt{
			Word: word(ParamExp{Short: true, Text: "i"}),
			List: []PatternList{
				{
					Patterns: litWords("1"),
					Stmts:    litStmts("foo"),
				},
				{
					Patterns: litWords("2", "3*"),
					Stmts:    litStmts("bar"),
				},
			},
		},
	},
	{
		[]string{"foo | while read a; do b; done"},
		BinaryExpr{
			Op: OR,
			X:  litStmt("foo"),
			Y: stmt(WhileStmt{
				Conds:   []Stmt{litStmt("read", "a")},
				DoStmts: litStmts("b"),
			}),
		},
	},
	{
		[]string{"while read l; do foo || bar; done"},
		WhileStmt{
			Conds: []Stmt{litStmt("read", "l")},
			DoStmts: stmts(BinaryExpr{
				Op: LOR,
				X:  litStmt("foo"),
				Y:  litStmt("bar"),
			}),
		},
	},
	{
		[]string{"echo if while"},
		litCmd("echo", "if", "while"),
	},
	{
		[]string{"echo ${foo}if"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Text: "foo"}, lit("if")),
		}},
	},
	{
		[]string{"echo $if"},
		Command{Args: []Word{
			litWord("echo"),
			word(ParamExp{Short: true, Text: "if"}),
		}},
	},
	{
		[]string{"if; then; fi", "if\nthen\nfi"},
		IfStmt{},
	},
	{
		[]string{"while; do; done", "while\ndo\ndone"},
		WhileStmt{},
	},
	{
		[]string{"for i; do; done", "for i\ndo\ndone"},
		ForStmt{Name: lit("i")},
	},
}

func fullProg(v interface{}) (f File) {
	switch x := v.(type) {
	case []Stmt:
		f.Stmts = x
	case Stmt:
		f.Stmts = append(f.Stmts, x)
	case []Node:
		for _, n := range x {
			f.Stmts = append(f.Stmts, stmt(n))
		}
	case Node:
		f.Stmts = append(f.Stmts, stmt(x))
	}
	return
}

func setPos(t *testing.T, v interface{}, to Pos, diff bool) Node {
	set := func(p *Pos) {
		if diff && *p == to {
			t.Fatalf("Pos in %v (%T) is already %v", v, v, to)
		}
		*p = to
	}
	switch x := v.(type) {
	case []Stmt:
		for i := range x {
			setPos(t, &x[i], to, diff)
		}
	case *Stmt:
		set(&x.Position)
		x.Node = setPos(t, x.Node, to, diff)
		for i := range x.Redirs {
			set(&x.Redirs[i].OpPos)
			setPos(t, &x.Redirs[i].N, to, diff)
			setPos(t, &x.Redirs[i].Word, to, diff)
		}
	case Command:
		setPos(t, x.Args, to, diff)
		return x
	case []Word:
		for i := range x {
			setPos(t, &x[i], to, diff)
		}
	case *Word:
		setPos(t, x.Parts, to, diff)
	case []Node:
		for i := range x {
			x[i] = setPos(t, x[i], to, diff)
		}
	case *Lit:
		set(&x.ValuePos)
	case Lit:
		set(&x.ValuePos)
		return x
	case Subshell:
		set(&x.Lparen)
		set(&x.Rparen)
		setPos(t, x.Stmts, to, diff)
		return x
	case Block:
		set(&x.Lbrace)
		set(&x.Rbrace)
		setPos(t, x.Stmts, to, diff)
		return x
	case IfStmt:
		set(&x.If)
		set(&x.Fi)
		setPos(t, x.Conds, to, diff)
		setPos(t, x.ThenStmts, to, diff)
		for i := range x.Elifs {
			set(&x.Elifs[i].Elif)
			setPos(t, x.Elifs[i].Conds, to, diff)
			setPos(t, x.Elifs[i].ThenStmts, to, diff)
		}
		setPos(t, x.ElseStmts, to, diff)
		return x
	case WhileStmt:
		set(&x.While)
		set(&x.Done)
		setPos(t, x.Conds, to, diff)
		setPos(t, x.DoStmts, to, diff)
		return x
	case ForStmt:
		set(&x.For)
		set(&x.Done)
		setPos(t, &x.Name, to, diff)
		setPos(t, x.WordList, to, diff)
		setPos(t, x.DoStmts, to, diff)
		return x
	case DblQuoted:
		set(&x.Quote)
		setPos(t, x.Parts, to, diff)
		return x
	case BckQuoted:
		set(&x.Quote)
		setPos(t, x.Stmts, to, diff)
		return x
	case BinaryExpr:
		set(&x.OpPos)
		setPos(t, &x.X, to, diff)
		setPos(t, &x.Y, to, diff)
		return x
	case FuncDecl:
		setPos(t, &x.Name, to, diff)
		setPos(t, &x.Body, to, diff)
		return x
	case ParamExp:
		set(&x.Exp)
		return x
	case ArithmExp:
		set(&x.Exp)
		return x
	case CmdSubst:
		set(&x.Exp)
		setPos(t, x.Stmts, to, diff)
		return x
	case CaseStmt:
		set(&x.Case)
		set(&x.Esac)
		setPos(t, &x.Word, to, diff)
		for _, pl := range x.List {
			setPos(t, pl.Patterns, to, diff)
			setPos(t, pl.Stmts, to, diff)
		}
		return x
	case nil:
	default:
		panic(v)
	}
	return nil
}

func TestNodePos(t *testing.T) {
	p := Pos{
		Line:   12,
		Column: 34,
	}
	defaultPos = p
	allTests := tests
	for _, v := range []interface{}{
		Command{},
		Command{Args: []Word{
			{},
		}},
	} {
		allTests = append(allTests, testCase{nil, v})
	}
	for _, c := range allTests {
		want := fullProg(c.ast)
		setPos(t, want.Stmts, p, true)
		for _, s := range want.Stmts {
			if s.Pos() != p {
				t.Fatalf("Found unexpected Pos in %v", s)
			}
			n := s.Node
			if n != nil && n.Pos() != p {
				t.Fatalf("Found unexpected Pos in %v", n)
			}
		}
	}
}

func TestParseAST(t *testing.T) {
	for _, c := range tests {
		want := fullProg(c.ast)
		setPos(t, want.Stmts, Pos{}, false)
		for _, in := range c.strs {
			r := strings.NewReader(in)
			got, err := Parse(r, "")
			if err != nil {
				t.Fatalf("Unexpected error in %q: %v", in, err)
			}
			setPos(t, got.Stmts, Pos{}, true)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("AST mismatch in %q\nwant: %s\ngot:  %s\ndumps:\n%#v\n%#v",
					in, want, got, want, got)
			}
		}
	}
}

func TestPrintAST(t *testing.T) {
	for _, c := range tests {
		in := fullProg(c.ast)
		want := c.strs[0]
		got := in.String()
		if got != want {
			t.Fatalf("AST print mismatch\nwant: %s\ngot:  %s",
				want, got)
		}
	}
}
