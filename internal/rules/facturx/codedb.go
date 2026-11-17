package facturx

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

//go:generate go run ./gen

// The code list assertions of the Factur-X 1.09.2 schematrons of BASIC,
// EN 16931 and EXTENDED (FX-SCH-A-*, generated in codedb_gen.go): each
// checks that a coded value (an element, or one of its attributes) is in a
// list of the codedb file of the schematron, or empty. The lists are those
// of FNFE-MPE and FeRD, not the CEN ones (BR-CL-*): they admit some codes
// that the CEN lists of version 1.3.17 do not, and the reverse.
//
// In the schematron, all these assertions sit in one pattern, where a node
// is checked by the first rule whose context matches it only: a charge
// reason code with a list identifier other than 5153 is not checked at all,
// a total VAT amount (BT-110) whose currency is the invoice currency is
// checked against one list, and so on. codeRules keeps these rules in the
// order of the pattern, and checkCodes applies the first one that matches.

// codeOp is the operation of a codeCond.
type codeOp uint8

const (
	codeText      codeOp = iota + 1 // a node of path has the string value val
	codeAttr                        // the node has the attribute attr
	codeAttrValue                   // the attribute attr of the node is val
	codeAttrText                    // the attribute attr of the node is the string value of a node of path
	codeNot                         // sub[0] is false
	codeAnd                         // sub[0] and sub[1] are true
	codeOr                          // sub[0] or sub[1] is true
)

// codeCond is a predicate of the context of a rule, evaluated on a node.
// path is a path of elements relative to the node, by local name, possibly
// after parent steps (".."), or an absolute path ("/CrossIndustryInvoice/...").
type codeCond struct {
	op   codeOp
	path string
	attr string
	val  string
	sub  []codeCond
}

// codeStepCond is a predicate on the step of a context: on the ancestor at
// depth step of the node checked (0 for the root element).
type codeStepCond struct {
	step int
	cond codeCond
}

// codeRule is a rule of the pattern of the code list assertions. A rule
// without id takes the nodes it matches from the following rules of its
// path without checking them.
type codeRule struct {
	path string         // local names of the steps of the context, from the root element
	when []codeStepCond // predicates of the context
	id   string         // identifier of the assertion
	attr string         // attribute holding the code, "" for the element
	cl   int            // number of the list in the codedb file
	list int            // index of the list in codeLists
	msg  string         // message of the assertion
	ctx  string         // context of the rule, prefixes left out, for the catalog
}

// codeTrie indexes the rules of a profile by the steps of their path.
type codeTrie struct {
	next  map[string]*codeTrie
	rules []*codeRule // in the order of the pattern
}

var codeTries = sync.OnceValue(func() map[invoice.Profile]*codeTrie {
	m := map[invoice.Profile]*codeTrie{}
	for p, rs := range map[invoice.Profile][]codeRule{
		invoice.ProfileBasic:    codeRulesBasic,
		invoice.ProfileEN16931:  codeRulesEN16931,
		invoice.ProfileExtended: codeRulesExtended,
	} {
		root := &codeTrie{}
		for i := range rs {
			t := root
			for _, step := range strings.Split(rs[i].path, "/") {
				if t.next == nil {
					t.next = map[string]*codeTrie{}
				}
				if t.next[step] == nil {
					t.next[step] = &codeTrie{}
				}
				t = t.next[step]
			}
			t.rules = append(t.rules, &rs[i])
		}
		m[p] = root
	}
	return m
})

// checkCodes applies the code list assertions of the Factur-X schematron of
// the profile p (BASIC, EN 16931 or EXTENDED).
func (k *checker) checkCodes(p invoice.Profile) {
	t := codeTries()[p]
	if t == nil {
		return
	}
	var stack [32]*xmltree.Node
	k.walkCodes(t, k.d.Root, stack[:0], p)
}

func (k *checker) walkCodes(t *codeTrie, n *xmltree.Node, stack []*xmltree.Node, p invoice.Profile) {
	t = t.next[n.Name]
	if t == nil {
		return
	}
	stack = append(stack, n)
	for _, r := range t.rules {
		if !r.matches(stack) {
			continue
		}
		if r.id != "" {
			k.checkCode(r, n, p)
		}
		break
	}
	if t.next == nil {
		return
	}
	for _, c := range n.Children {
		k.walkCodes(t, c, stack, p)
	}
}

// matches reports whether the predicates of the context of r hold on the
// ancestors of the node checked, stack[len(stack)-1].
func (r *codeRule) matches(stack []*xmltree.Node) bool {
	for _, w := range r.when {
		if !w.cond.eval(stack[w.step]) {
			return false
		}
	}
	return true
}

// checkCode applies the assertion of r to n.
func (k *checker) checkCode(r *codeRule, n *xmltree.Node, p invoice.Profile) {
	var v string
	if r.attr != "" {
		v, _ = n.LookupAttr(r.attr)
	} else {
		v = stringValue(n)
	}
	if v == "" {
		return
	}
	if _, ok := slices.BinarySearch(codeLists[r.list], v); ok {
		return
	}
	what := n.Name
	if r.attr != "" {
		what = "@" + r.attr
	}
	k.fail(r.id, n, "%q (%s) is not in code list %d of the Factur-X %s schematron: %s", v, what, r.cl, codeProfileName(p), r.msg)
}

// codeProfileName returns the name of the profile in the messages.
func codeProfileName(p invoice.Profile) string {
	if p == invoice.ProfileEN16931 {
		return "EN 16931"
	}
	return p.String()
}

// eval evaluates c on node n.
func (c *codeCond) eval(n *xmltree.Node) bool {
	switch c.op {
	case codeText:
		return hasValue(n, c.path, c.val)
	case codeAttr:
		_, ok := n.LookupAttr(c.attr)
		return ok
	case codeAttrValue:
		v, ok := n.LookupAttr(c.attr)
		return ok && v == c.val
	case codeAttrText:
		v, ok := n.LookupAttr(c.attr)
		return ok && hasValue(n, c.path, v)
	case codeNot:
		return !c.sub[0].eval(n)
	case codeAnd:
		return c.sub[0].eval(n) && c.sub[1].eval(n)
	case codeOr:
		return c.sub[0].eval(n) || c.sub[1].eval(n)
	}
	panic(fmt.Sprintf("facturx: unknown code condition %d", c.op))
}

// hasValue reports whether an element that path selects from n (see
// codeCond) has the string value v: the general comparison path = v of
// XPath.
func hasValue(n *xmltree.Node, path, v string) bool {
	if rest, ok := strings.CutPrefix(path, "/"); ok {
		for n.Parent != nil {
			n = n.Parent
		}
		first, tail, _ := strings.Cut(rest, "/")
		if n.Name != first {
			return false
		}
		path = tail
	}
	return pathValue(n, path, v)
}

func pathValue(n *xmltree.Node, path, v string) bool {
	if path == "" {
		return stringValue(n) == v
	}
	step, rest, _ := strings.Cut(path, "/")
	switch step {
	case ".":
		return pathValue(n, rest, v)
	case "..":
		return n.Parent != nil && pathValue(n.Parent, rest, v)
	}
	for _, c := range n.Children {
		if c.Name == step && pathValue(c, rest, v) {
			return true
		}
	}
	return false
}

// stringValue returns the string value of an element in XPath: the
// character data of its descendants.
func stringValue(n *xmltree.Node) string {
	if len(n.Children) == 0 {
		return n.Text
	}
	var b strings.Builder
	var walk func(*xmltree.Node)
	walk = func(m *xmltree.Node) {
		b.WriteString(m.Text)
		for _, c := range m.Children {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// codeCatalog returns the catalog entries of the code list assertions.
func codeCatalog() []rule {
	var out []rule
	for _, x := range []struct {
		mask  uint8
		label string
		rules []codeRule
	}{
		{mBasic, "BASIC", codeRulesBasic},
		{mEN, "EN 16931", codeRulesEN16931},
		{mExt, "EXTENDED", codeRulesExtended},
	} {
		for _, r := range x.rules {
			if r.id == "" {
				continue
			}
			what := "value"
			if r.attr != "" {
				what = "@" + r.attr
			}
			out = append(out, rule{r.id, rules.Error, x.mask, r.id,
				fmt.Sprintf("Factur-X %s: the %s of %s is empty or in code list %d of the schematron.", x.label, what, r.ctx, r.cl)})
		}
	}
	return out
}
